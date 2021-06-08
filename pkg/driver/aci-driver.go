package driver

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"reflect"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/beevik/guid"
	"github.com/cnabio/cnab-go/bundle"
	"github.com/cnabio/cnab-go/driver"
	"github.com/docker/distribution/reference"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	az "github.com/deislabs/cnab-azure-driver/pkg/azure"

	"os"
	"strings"
	"time"
)

const (
	userAgentPrefix      = "azure-cnab-driver"
	fileMountPoint       = "/mnt/BundleFiles"
	fileMountName        = "bundlefilevolume"
	stateMountName       = "state"
	stateMountPoint      = "/cnab/state"
	cnabOutputDirName    = "outputs"
	cnabOutputMountPoint = "/cnab/app/"

	// We could have a more complex regex for the subscription ID but
	// we parse that anyway to ensure validity so we can keep the regex here simple.
	// Azure subscription ids are assumed to be a 36 char alphanumeric sequence
	// with `-` separators between blocks e.g. `28fb4867-4cef-43ed-9637-b678cd6b2ce3`
	// The resource group name regex is taken from:
	// https://docs.microsoft.com/en-us/rest/api/resources/resource-groups/create-or-update
	// However, this regex does not account for names ending with periods so we
	// validate that separatley too.
	azureSubscriptionScopeRegexPattern  = "^/subscriptions/[a-z0-9-]{36}$"
	azureResourceGroupScopeRegexPattern = "^/subscriptions/[a-z0-9-]{36}/resourceGroups/[-\\w\\._\\(\\)]+$"
)

// aciDriver runs Docker and OCI invocation images in ACI
type aciDriver struct {
	deleteACIResources      bool
	subscriptionID          string
	clientID                string
	clientSecret            string
	tenantID                string
	applicationID           string
	aciRG                   string
	createRG                bool
	aciLocation             string
	aciName                 string
	msiType                 string
	msiResource             azure.Resource
	systemMSIScope          string
	systemMSIRole           string
	propagateCredentials    bool
	userMSIResourceID       string
	useSPForACR             bool
	imageRegistryUser       string
	imageRegistryPassword   string
	hasStateVolumeInfo      bool
	mountStateVolume        bool
	stateFileShare          string
	stateStorageAccountName string
	stateStorageAccountKey  string
	statePath               string
	stateMountPoint         string
	userAgent               string
	loginInfo               az.LoginInfo
	hasOutputs              bool
	deleteOutputs           bool
	debugContainer          bool
}

// Config returns the ACI driver configuration options
func (d *aciDriver) Config() map[string]string {
	return map[string]string{
		"CNAB_AZURE_VERBOSE":                            "Increase verbosity. true, false are supported values",
		"CNAB_AZURE_CLIENT_ID":                          "AAD Client ID for Azure account authentication - used to authenticate to Azure for ACI creation",
		"CNAB_AZURE_CLIENT_SECRET":                      "AAD Client Secret for Azure account authentication - used to authenticate to Azure for ACI creation",
		"CNAB_AZURE_TENANT_ID":                          "Azure AAD Tenant Id Azure account authentication - used to authenticate to Azure for ACI creation",
		"CNAB_AZURE_SUBSCRIPTION_ID":                    "Azure Subscription Id - this is the subscription to be used for ACI creation, if not specified the default subscription is used",
		"CNAB_AZURE_APP_ID":                             "Azure Application Id - this is the application to be used to authenticate to Azure",
		"CNAB_AZURE_RESOURCE_GROUP":                     "The name of the existing Resource Group to create the ACI instance in, if not specified a Resource Group will be created",
		"CNAB_AZURE_LOCATION":                           "The location to create the ACI Instance in",
		"CNAB_AZURE_NAME":                               "The name of the ACI instance to create - if not specified a name will be generated",
		"CNAB_AZURE_DELETE_RESOURCES":                   "Delete RG and ACI instance created - default is true useful to set to false for debugging - only deletes RG if it was created by the driver",
		"CNAB_AZURE_MSI_TYPE":                           "This can be set to user or system",
		"CNAB_AZURE_SYSTEM_MSI_ROLE":                    "The role to be asssigned to System MSI User - used if CNAB_AZURE_ACI_MSI_TYPE == system, if this is null or empty then the role defaults to contributor",
		"CNAB_AZURE_SYSTEM_MSI_SCOPE":                   "The scope to apply the role to System MSI User - will attempt to set scope to the  Resource Group that the ACI Instance is being created in if not set",
		"CNAB_AZURE_USER_MSI_RESOURCE_ID":               "The resource Id of the MSI User - required if CNAB_AZURE_ACI_MSI_TYPE == User ",
		"CNAB_AZURE_PROPAGATE_CREDENTIALS":              "If this is set to true the credentials used to Launch the Driver are propagated to the invocation image in an ENV variable, the  CNAB_AZURE prefix will be relaced with AZURE_, default is false",
		"CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH": "If this is set to true the CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET are also used for authentication to ACR",
		"CNAB_AZURE_REGISTRY_USERNAME":                  "The username for authenticating to the container registry",
		"CNAB_AZURE_REGISTRY_PASSWORD":                  "The password for authenticating to the container registry",
		"CNAB_AZURE_STATE_FILESHARE":                    "The File Share for Azure State volume",
		"CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME":         "The Storage Account for the Azure State File Share",
		"CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY":          "The Storage Key for the Azure State File Share",
		"CNAB_AZURE_STATE_MOUNT_POINT":                  "The mount point location for state volume",
		"CNAB_AZURE_DELETE_OUTPUTS_FROM_FILESHARE":      "Any Outputs Created in the fileshare are deleted on completion",
		"CNAB_AZURE_DEBUG_CONTAINER":                    "Replaces /cnab/app/run with tail -f /dev/null so that container can be connected to and debugged",
	}
}

// NewACIDriver creates a new ACI Driver instance
func NewACIDriver(version string) (driver.Driver, error) {
	d := &aciDriver{
		msiResource: azure.Resource{},
	}
	d.userAgent = fmt.Sprintf("%s-%s", userAgentPrefix, version)
	config := make(map[string]string)
	for env := range d.Config() {
		config[env] = os.Getenv(env)
	}
	if err := d.processConfiguration(config); err != nil {
		return nil, err
	}

	return d, nil
}
func (d *aciDriver) processConfiguration(config map[string]string) error {

	// TODO retrieve settings from CloudShell

	// This controls the deletion of ARM resources when the driver is complete, by default all resources that are created are cleaned up
	d.deleteACIResources = true
	if len(config["CNAB_AZURE_DELETE_RESOURCES"]) > 0 && (strings.ToLower(config["CNAB_AZURE_DELETE_RESOURCES"]) == "false") {
		d.deleteACIResources = false
	}
	log.Debug("Delete Resources: ", d.deleteACIResources)

	// Azure AAD Client Id for authenticating to Azure
	d.clientID = config["CNAB_AZURE_CLIENT_ID"]
	log.Debug("Client ID: ", d.clientID)

	// Azure AAD Client Secret for authenticating to Azure
	d.clientSecret = config["CNAB_AZURE_CLIENT_SECRET"]
	log.Debug("Client Secret Set: ", len(d.clientSecret) > 0)

	//Validate that both of Client Id and Client Secret are set
	clientCreds, err := checkAllOrNoneSet(config, []string{"CNAB_AZURE_CLIENT_ID", "CNAB_AZURE_CLIENT_SECRET"})
	if err != nil {
		return err
	}

	// Azure Tenant Id for authenticating to Azure
	d.tenantID = config["CNAB_AZURE_TENANT_ID"]
	log.Debug("Tenant ID: ", d.tenantID)

	// Azure Application Id to be used with device code auth flow
	d.applicationID = config["CNAB_AZURE_APP_ID"]
	log.Debug("Application ID: ", d.applicationID)
	appID := len(d.applicationID) > 0

	// SPN and appId are mutually exclusive
	if clientCreds && appID {
		return errors.New("either CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID should be set not both")
	}

	// TenantId is required when client credentials or CNAB_AZURE_APP_ID is set
	if (clientCreds || appID) && len(d.tenantID) == 0 {
		if az.IsInCloudShell() {
			d.tenantID = az.GetTenantIDFromCliProfile()
		}
		if len(d.tenantID) == 0 {
			return errors.New("CNAB_AZURE_TENANT_ID should be set when CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID are set")
		}
	}

	// TenantId should not be set if client creds or app id not set
	if !clientCreds && !appID && len(d.tenantID) > 0 {
		return errors.New("CNAB_AZURE_TENANT_ID should not be set when CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID are not set")
	}

	// Azure Subscription Id to create resources to run invocation image in - if this is not set then the first subscription found will be used
	d.subscriptionID = config["CNAB_AZURE_SUBSCRIPTION_ID"]
	if len(d.subscriptionID) == 0 {
		d.subscriptionID = az.GetSubscriptionIDFromCliProfile()
	}
	log.Debug("Subscription ID: ", d.subscriptionID)

	// Check to see if an resource group name has been set, if not then a location must be set , if an resource group name is set and no location is used then the resource group must already exist and the location of he resource group will be used for the resources
	d.aciRG = config["CNAB_AZURE_RESOURCE_GROUP"]
	log.Debug("Resource Group: ", d.aciRG)
	d.aciLocation = strings.ToLower(strings.Replace(config["CNAB_AZURE_LOCATION"], " ", "", -1))
	log.Debug("Location: ", d.aciLocation)

	if len(d.aciRG) == 0 && len(d.aciLocation) == 0 && az.IsInCloudShell() {
		d.aciRG, d.aciLocation = az.TryGetRGandLocation()
	}
	if len(d.aciRG) == 0 && len(d.aciLocation) == 0 {
		return errors.New("ACI Driver requires CNAB_AZURE_LOCATION environment variable or an existing Resource Group in CNAB_AZURE_RESOURCE_GROUP")
	}

	// If Resource group name is not set generate a unique name
	d.createRG = len(d.aciRG) == 0
	if d.createRG {
		d.aciRG = fmt.Sprintf("cnab-azure-%s", uuid.New().String())
		log.Debug("New Resource Group : ", d.aciRG)
	}

	// If aci driver name is not set generate a unique aci name
	d.aciName = config["CNAB_AZURE_NAME"]
	if len(d.aciName) == 0 {
		d.aciName = fmt.Sprintf("cnab-azure-%s", uuid.New().String())
	}

	log.Debug("Generated ACI Name: ", d.aciName)
	// Check that if MSI type is set it is either user or system, if it is user then there must also be a valid resource id set for the user MSI
	if len(config["CNAB_AZURE_MSI_TYPE"]) > 0 {
		log.Debug("MSI Type: ", config["CNAB_AZURE_MSI_TYPE"])
		switch strings.ToLower(config["CNAB_AZURE_MSI_TYPE"]) {
		case "system":
			d.msiType = "system"
			d.systemMSIRole = "Contributor"
			if len(config["CNAB_AZURE_SYSTEM_MSI_ROLE"]) > 0 {
				d.systemMSIRole = config["CNAB_AZURE_SYSTEM_MSI_ROLE"]
			}
			log.Debug("System MSI Role: ", d.systemMSIRole)

			d.systemMSIScope = ""
			if len(config["CNAB_AZURE_SYSTEM_MSI_SCOPE"]) > 0 {
				d.systemMSIScope = config["CNAB_AZURE_SYSTEM_MSI_SCOPE"]
				err := validateMSIScope(d.systemMSIScope)
				if err != nil {
					return fmt.Errorf("CNAB_AZURE_SYSTEM_MSI_SCOPE environment variable parsing error: %v", err)
				}
				log.Debugf("System MSI Scope: %s", d.systemMSIScope)
			} else {
				log.Debugf("System MSI Scope Not Set will Scope to RG: %s ", d.aciRG)
			}

		case "user":
			d.msiType = "user"
			d.userMSIResourceID = config["CNAB_AZURE_USER_MSI_RESOURCE_ID"]
			log.Debug("User MSI Resource ID: e", d.userMSIResourceID)

			if len(d.userMSIResourceID) == 0 {
				return errors.New("ACI Driver requires CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable when CNAB_AZURE_MSI_TYPE is set to user")
			}

			resource, err := azure.ParseResourceID(d.userMSIResourceID)
			if err != nil {
				return fmt.Errorf("CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable parsing error: %v", err)
			}

			if strings.ToLower(resource.Provider) != "microsoft.managedidentity" || strings.ToLower(resource.ResourceType) != "userassignedidentities" {
				return fmt.Errorf("CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: %s/%s", resource.Provider, resource.ResourceType)
			}

			d.msiResource = resource

		default:
			return fmt.Errorf("CNAB_AZURE_MSI_TYPE environment variable unknown value: %s", config["CNAB_AZURE_MSI_TYPE"])
		}
	}

	// Propagation of Credentials enables the flow of Azure credentials from the ACI_DRIVER to the invocation image
	d.propagateCredentials = len(config["CNAB_AZURE_PROPAGATE_CREDENTIALS"]) > 0 && strings.ToLower(config["CNAB_AZURE_PROPAGATE_CREDENTIALS"]) == "true"
	log.Debug("Propagate Credentials: ", d.propagateCredentials)

	// Credentials to be used for container registry for the invocation image
	d.imageRegistryUser = config["CNAB_AZURE_REGISTRY_USERNAME"]
	d.imageRegistryPassword = config["CNAB_AZURE_REGISTRY_PASSWORD"]

	// Both CNAB_AZURE_REGISTRY_USERNAME and CNAB_AZURE_REGISTRY_PASSWORD are required
	registryCredsSet, err := checkAllOrNoneSet(config, []string{"CNAB_AZURE_REGISTRY_USERNAME", "CNAB_AZURE_REGISTRY_PASSWORD"})
	if err != nil {
		return err
	}

	// CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH enables the SPN to also be used for authenticating to the registry that contains the invocation image
	d.useSPForACR = len(config["CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH"]) > 0 && strings.ToLower(config["CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH"]) == "true"
	if d.useSPForACR {
		if registryCredsSet {
			return errors.New("CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH should not be set if CNAB_AZURE_REGISTRY_USERNAME and CNAB_AZURE_REGISTRY_PASSWORD are set")
		}
		if len(d.clientID) == 0 || len(d.clientSecret) == 0 {
			return errors.New("Both CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET should be set when setting CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH")
		}
		d.imageRegistryPassword = d.clientSecret
		d.imageRegistryUser = d.clientID
	}
	d.mountStateVolume = false
	// CNAB_AZURE_STATE_* allows an Azure File Share to be mounted to the invocation image sto be used for instance state
	// TODO Allow empty storage account key and do runtime lookup
	d.hasStateVolumeInfo, err = checkAllOrNoneSet(config, []string{"CNAB_AZURE_STATE_FILESHARE", "CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME", "CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY"})
	if err != nil {
		return err
	}

	if d.hasStateVolumeInfo {
		d.stateFileShare = config["CNAB_AZURE_STATE_FILESHARE"]
		d.stateStorageAccountName = config["CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME"]
		d.stateStorageAccountKey = config["CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY"]
		d.mountStateVolume = true
	}

	// set state mount point to default if not set
	if len(config["CNAB_AZURE_STATE_MOUNT_POINT"]) > 0 {
		if !path.IsAbs(config["CNAB_AZURE_STATE_MOUNT_POINT"]) {
			return fmt.Errorf("value (%s) of CNAB_AZURE_STATE_MOUNT_POINT is not an absolute path", config["CNAB_AZURE_STATE_MOUNT_POINT"])
		}
		d.stateMountPoint = path.Clean(strings.TrimSpace(strings.TrimSuffix(config["CNAB_AZURE_STATE_MOUNT_POINT"], "/")))
		log.Debugf("State Mount Point: %v", d.stateMountPoint)
		if d.stateMountPoint == "." || d.stateMountPoint == "/" {
			return errors.New("CNAB_AZURE_STATE_MOUNT_POINT should not be root path")
		}
	} else {
		d.stateMountPoint = stateMountPoint
	}

	d.deleteOutputs = !(len(config["CNAB_AZURE_DELETE_OUTPUTS_FROM_FILESHARE"]) > 0 && strings.ToLower(config["CNAB_AZURE_DELETE_OUTPUTS_FROM_FILESHARE"]) == "false")
	d.debugContainer = len(config["CNAB_AZURE_DEBUG_CONTAINER"]) > 0 && strings.ToLower(config["CNAB_AZURE_DEBUG_CONTAINER"]) == "true"

	return nil
}

// Validate a provided MSI scope matches one of the supported formats:
// * /subscriptions/<subscriptionID>
// * /subscriptions/<subscriptionID>/resourceGroups/<resourceGroupName>
// * /subscriptions/<subscriptionID>/resourceGroups/<resourceGroupName>/providers/...
func validateMSIScope(scope string) error {
	parts := strings.Split(scope, "/") // Leading slash '/' in scope adds an empty part.
	if len(parts) < 3 {
		return errors.New("invalid msi scope, scope must start with /subscriptions/<subscriptionID>")
	}

	// Azure subscription ids should be represented as 32 bit guids.
	subID := parts[2]
	if _, err := guid.ParseString(subID); err != nil {
		return fmt.Errorf("invalid msi scope, %w", err)
	}

	// format: /subscriptions/<subscriptionID>
	matchSubScope, _ := regexp.MatchString(azureSubscriptionScopeRegexPattern, scope)
	if matchSubScope && len(parts) == 3 {
		return nil
	}

	// format: /subscriptions/<subscriptionID>/resourceGroups/<resourceGroupName>
	matchGroupScope, _ := regexp.MatchString(azureResourceGroupScopeRegexPattern, scope)
	if matchGroupScope && len(parts) == 5 {
		lastChar := scope[len(scope)-1:]
		if lastChar == "." {
			return errors.New("invalid msi scope, resource group name cannot end in a '.' character")
		}
		return nil
	}

	// format: /subscriptions/<subscriptionID>/resourceGroups/<resourceGroupName>/providers/...
	if _, err := azure.ParseResourceID(scope); err != nil {
		return fmt.Errorf("invalid msi scope, %w", err)
	}
	return nil
}

// Checks that all or none of a set of configuration values are set
func checkAllOrNoneSet(config map[string]string, items []string) (bool, error) {
	var length = 0
	for i := 0; i < len(items); i++ {
		if val, exists := config[items[i]]; exists {
			length += len(val)
		} else {
			return false, fmt.Errorf("Config Item %s does not exist", items[i])
		}
	}
	if length > 0 {
		for i := 0; i < len(items); i++ {
			if len(config[items[i]]) == 0 {
				return false, fmt.Errorf("All of %s must be set when one is set. %s is not set", strings.Join(items, ","), items[i])
			}
		}
		return true, nil
	}
	return false, nil
}

// Run executes the ACI driver
func (d *aciDriver) Run(op *driver.Operation) (driver.OperationResult, error) {
	return d.exec(op)
}

// Handles indicates that the ACI driver supports "docker" and "oci"
func (d *aciDriver) Handles(dt string) bool {
	return dt == driver.ImageTypeDocker || dt == driver.ImageTypeOCI
}

func (d *aciDriver) exec(op *driver.Operation) (driver.OperationResult, error) {

	var err error
	operationResult := driver.OperationResult{
		Outputs: map[string]string{},
	}

	// Check that there is a state volume if needed
	d.hasOutputs = len(op.Outputs) > 0
	if d.hasOutputs && !d.hasStateVolumeInfo && az.IsInCloudShell() {
		log.Debug("Getting File share info from CloudShell")
		fileshare, err := az.GetCloudDriveDetails(d.userAgent)
		if err != nil {
			return operationResult, fmt.Errorf("Bundle has outputs and no volume mounted for state, failed to get clouddrive details ,set CNAB_AZURE_STATE_* variables so that state can be retrieved: %v", err)
		}
		log.Debug("State File Share: ", fileshare.Name)
		log.Debug("State Storage Account Name: ", fileshare.StorageAccountName)
		d.stateFileShare = fileshare.Name
		d.stateStorageAccountName = fileshare.StorageAccountName
		d.stateStorageAccountKey = fileshare.StorageAccountKey
		d.mountStateVolume = true
		d.hasStateVolumeInfo = true
	}

	if d.hasOutputs && !d.hasStateVolumeInfo {
		return operationResult, errors.New("Bundle has outputs no volume mounted for state, set CNAB_AZURE_STATE_* variables so that state can be retrieved")
	}

	if d.hasOutputs && d.deleteOutputs {
		defer d.deleteOutputsFromFileShare(op, &operationResult)
	}

	d.loginInfo, err = az.LoginToAzure(d.clientID, d.clientSecret, d.tenantID, d.applicationID)
	if err != nil {
		return operationResult, fmt.Errorf("cannot Login To Azure: %v", err)
	}

	err = d.setAzureSubscriptionID()
	if err != nil {
		return operationResult, fmt.Errorf("cannot set Azure subscription: %v", err)
	}

	err = d.runInvocationImageUsingACI(op)
	if err != nil {
		return operationResult, fmt.Errorf("running invocation instance using ACI failed: %v", err)
	}

	// Get any outputs
	return d.getOutputs(op, &operationResult)
}
func (d *aciDriver) deleteOutputsFromFileShare(op *driver.Operation, operationResult *driver.OperationResult) {
	fmt.Println("Deleting Outputs from Azure FileShare")
	afs, err := az.NewFileShare(d.stateStorageAccountName, d.stateStorageAccountKey, d.stateFileShare)
	if err != nil {
		fmt.Printf("Error creating AzureFileShare object to delete outputs: %v\n", err)
		return
	}
	cnabOutputPrefix := cnabOutputMountPoint + cnabOutputDirName
	for _, fullOutputName := range op.Outputs {
		log.Debugf("Deleting output %s from fileshare", fullOutputName)
		outputName := strings.TrimPrefix(fullOutputName, cnabOutputPrefix+"/")
		fileName := fmt.Sprintf("%s/%s/%s", d.statePath, cnabOutputDirName, outputName)
		_, err := afs.DeleteFileFromShare(fileName)
		if err != nil {
			fmt.Printf("Error deleting output %s from fileshare:%v\n", fullOutputName, err)
		}
	}
}
func (d *aciDriver) getOutputs(op *driver.Operation, operationResult *driver.OperationResult) (driver.OperationResult, error) {
	if d.hasOutputs {
		fmt.Println("Retreiving Outputs")
		afs, err := az.NewFileShare(d.stateStorageAccountName, d.stateStorageAccountKey, d.stateFileShare)
		if err != nil {
			return *operationResult, fmt.Errorf("Error creating AzureFileShare structure: %v", err)
		}
		cnabOutputPrefix := cnabOutputMountPoint + cnabOutputDirName
		for outputPath, fullOutputName := range op.Outputs {
			log.Debugf("Processing output for: %s", fullOutputName)
			// Output might not apply to this action
			if output := op.Bundle.Outputs[fullOutputName]; output.AppliesTo(op.Action) {
				log.Debugf("Checking for output for: %s", fullOutputName)
				outputName := strings.TrimPrefix(fullOutputName, cnabOutputPrefix+"/")
				fileName := fmt.Sprintf("%s/%s/%s", d.statePath, cnabOutputDirName, outputName)
				exists, err := afs.CheckIfFileExists(fileName)
				if err != nil {
					return *operationResult, fmt.Errorf("Error checking file exists %s from AzureFileShare: %v", fileName, err)
				}
				if !exists {
					log.Debugf("Output File: %s does not exist", fileName)
					// Output may not exist cnab-go command driver checks for default values so no need to check here
					continue
				}
				log.Debugf("Reading output for file: %s", fileName)
				content, err := afs.ReadFileFromShare(fileName)
				if err != nil {
					return *operationResult, fmt.Errorf("Error reading output %s from AzureFileShare: %v", fileName, err)
				}
				operationResult.Outputs[outputPath] = content
			}
		}
	}

	return *operationResult, nil
}

func (d *aciDriver) setAzureSubscriptionID() error {
	subscriptionsClient, err := az.GetSubscriptionsClient(d.loginInfo.Authorizer, d.userAgent)
	if err != nil {
		return fmt.Errorf("Error getting Subscription Client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if len(d.subscriptionID) != 0 {
		log.Debugf("Checking if Subscription ID: %s exists", d.subscriptionID)
		result, err := subscriptionsClient.Get(ctx, d.subscriptionID)
		if err != nil {
			if result.StatusCode == 404 {
				return fmt.Errorf("Subscription ID: %s not found", d.subscriptionID)
			}

			return fmt.Errorf("Attempt to Get Subscription Failed: %v", err)
		}

	} else {
		log.Debug("No Subscription ID set choosing first one available")
		result, err := subscriptionsClient.ListComplete(ctx)
		if err != nil {
			return fmt.Errorf("Attempt to List Subscriptions Failed: %v", err)
		}

		err = result.NextWithContext(ctx)
		if err != nil {
			return fmt.Errorf("Attempt to Get Subscription Failed: %v", err)
		}

		// Just choose the first subscription
		if result.NotDone() {
			subscriptionID := *result.Value().SubscriptionID
			log.Debug("Setting Subscription ID to: ", subscriptionID)
			d.subscriptionID = subscriptionID
		} else {
			return errors.New("Cannot find a subscription for account")
		}

	}

	return nil
}

func (d *aciDriver) runInvocationImageUsingACI(op *driver.Operation) error {

	// TODO Check that image is a type and platform that can be executed by ACI
	fmt.Println("Creating Azure Container Instance To Execute Bundle")
	// GET ACI Config
	image := imageWithDigest(op.Image)
	ref, err := reference.ParseAnyReference(image)
	if err != nil {
		return fmt.Errorf("Failed to parse image reference: %s error: %v", image, err)
	}

	var domain string
	if named, ok := ref.(reference.Named); ok {
		domain = reference.Domain(named)
	}

	// SPN details are for Azure registry only
	if d.useSPForACR && !strings.HasSuffix(domain, "azurecr.io") {
		return fmt.Errorf("Cannot use Service Principal as credentials for non Azure registry : %s", domain)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	groupsClient, err := az.GetGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	if err != nil {
		return fmt.Errorf("Error getting Groups Client Client: %v", err)
	}

	if !d.createRG {
		rg, err := groupsClient.Get(ctx, d.aciRG)
		if err != nil {
			return fmt.Errorf("Checking for existing resource group %s failed with error: %v", d.aciRG, err)
		}

		// If no location was provided then use the location of the Resource Group
		if len(d.aciLocation) == 0 {
			log.Debug("Setting ACI Location to RG Location:", *rg.Location)
			d.aciLocation = *rg.Location
		}

	}

	// Check that location supports ACI

	providersClient, err := az.GetProvidersClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	if err != nil {
		return fmt.Errorf("Error getting Providers Accounts Client: %v", err)
	}

	provider, err := providersClient.Get(ctx, "Microsoft.ContainerInstance", "")
	if err != nil {
		return fmt.Errorf("Error getting provider details for ACI: %v", err)
	}

	for _, t := range *provider.ResourceTypes {
		if *t.ResourceType == "ContainerGroups" {
			if !locationIsAvailable(d.aciLocation, *t.Locations) {
				return fmt.Errorf("ACI driver location is invalid: %v", d.aciLocation)
			}

		}

	}

	if d.createRG {
		// If in cloudshell check that RG can be created
		// TODO update so that this check works outside cloudshell
		if az.IsInCloudShell() {
			scope := fmt.Sprintf("subscriptions/%s", d.subscriptionID)
			log.Debug("Checking permission to create resource group at scope: ", scope)
			if canCreateRG, err := az.CheckCanAccessResource("Microsoft.Resources/subscriptions/resourceGroups/write", scope); err != nil || !canCreateRG {
				// TODO This check is producing false negatives (user with access is getting Not Allowed just log response for now)
				if err != nil {
					log.Debug(fmt.Sprintf("Failed checking access for RG write access at scope %s Error: %v", scope, err))
					//return fmt.Errorf("Failed checking access for RG write access at scope %s Error: %v", scope, err)
				}
				log.Debug(fmt.Sprintf("You do not have permission to create resource groups in subscription %s, you can set a default subscription using az configure -d group=<rg name> or set the Resource Group name in environment variable CNAB_AZURE_RESOURCE_GROUP", d.subscriptionID))
				//return fmt.Errorf("You do not have permission to create resource groups in subscription %s, you can set a default subscription using az configure -d group=<rg name> or set the Resource Group name in environment variable CNAB_AZURE_RESOURCE_GROUP", d.subscriptionID)
			}
		}

		log.Debug("Creating Resource Group: ", d.aciRG)
		_, err := groupsClient.CreateOrUpdate(
			ctx,
			d.aciRG,
			resources.Group{
				Location: &d.aciLocation,
			})
		if err != nil {
			return fmt.Errorf("Failed to create resource group: %v", err)
		}

		defer func() {
			if d.deleteACIResources {
				log.Debug("Deleting Resource Group: ", d.aciRG)
				future, err := groupsClient.Delete(ctx, d.aciRG)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to execute delete resource group %s error: %v\n", d.aciRG, err)
				}

				err = future.WaitForCompletionRef(ctx, groupsClient.Client)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to delete resource group %s error: %v\n", d.aciRG, err)
				} else {
					log.Debug("Deleted Resource Group ", d.aciRG)
				}

			}

		}()
	}

	// Check if permission to create ACI and permission
	if az.IsInCloudShell() {
		scope := fmt.Sprintf("subscriptions/%s/resourceGroups/%s", d.subscriptionID, d.aciRG)
		log.Debug("Checking permission to create ACI at scope: ", scope)
		if canCreateCG, err := az.CheckCanAccessResource("Microsoft.ContainerInstance/containerGroups/write", scope); err != nil || !canCreateCG {
			// TODO This check is producing false negatives (user with access is getting Not Allowed just log response for now)
			if err != nil {
				log.Debug(fmt.Sprintf("Failed checking access for Container Group Write access at scope %s Error: %v", scope, err))
				//return fmt.Errorf("Failed checking access for Container Group Write access at scope %s Error: %v", scope, err)
			}
			log.Debug(fmt.Sprintf("You do not have permission to create container groups in resource group %s in subscription %s", d.aciRG, d.subscriptionID))
			//return fmt.Errorf("You do not have permission to create container groups in resource group %s in subscription %s", d.aciRG, d.subscriptionID)
		}
	}

	var mounts []containerinstance.VolumeMount
	var volumes []containerinstance.Volume

	// ACI does not support file copy
	// files are mounted into the container in a secrets volume and invocationImage Entry point is modified to process the files before run cmd is invoked

	hasFiles := false
	log.Debug("Bundle Has File Inputs:", hasFiles)
	if len(op.Files) > 0 {

		// TODO Check that the run cmd is "/cnab/app/run"

		hasFiles = true
		secretMount := containerinstance.VolumeMount{
			MountPath: to.StringPtr(fileMountPoint),
			Name:      to.StringPtr(fileMountName),
		}
		mounts = append(mounts, secretMount)
		secrets := make(map[string]*string)
		secretVolume := containerinstance.Volume{
			Name:   to.StringPtr(fileMountName),
			Secret: secrets,
		}
		volumes = append(volumes, secretVolume)
		i := 0
		for k, v := range op.Files {
			log.Debug("Processing File Input: ", k)
			secrets[fmt.Sprintf("path%d", i)] = to.StringPtr(base64.StdEncoding.EncodeToString([]byte(k)))
			secrets[fmt.Sprintf("value%d", i)] = to.StringPtr(base64.StdEncoding.EncodeToString([]byte(v)))
			i++
		}
	}

	// Create ACI Instance

	var env []containerinstance.EnvironmentVariable
	env = d.createMSIEnvVars(env)
	if d.propagateCredentials {
		env = d.createAzureEnvironmentEnvVars(env)
		// Only propagate credentials if not using MSI
		if len(d.msiType) == 0 {
			env, err = d.createCredentialEnvVars(env)
		}

		if err != nil {
			return fmt.Errorf("Failed to create environment variables for Credentials:%v", err)
		}

	}

	for k, v := range op.Environment {
		// Need to check if any of the env variables already exist in case any propagated credentials are being overridden
		for _, ev := range env {
			if k == *ev.Name {
				ev.SecureValue = to.StringPtr(strings.Replace(v, "'", "''", -1))
				log.Debug("Updating Container Group Environment Variable: Name: ", k)
				continue
			}

		}
		env = append(env, containerinstance.EnvironmentVariable{
			Name:        to.StringPtr(k),
			SecureValue: to.StringPtr(strings.Replace(v, "'", "''", -1)),
		})
		log.Debug("Setting Container Group Environment Variable: Name: ", k)
	}
	var volume = containerinstance.Volume{}
	var volumeMount = containerinstance.VolumeMount{}
	if d.mountStateVolume {
		d.statePath = fmt.Sprintf("%s/%s", strings.ToLower(op.Bundle.Name), strings.ToLower(op.Installation))
		statePath := fmt.Sprintf("%s/%s", d.stateMountPoint, d.statePath)
		log.Debug("State Path: ", statePath)
		env = append(env, containerinstance.EnvironmentVariable{
			Name:  to.StringPtr("STATE_PATH"),
			Value: to.StringPtr(statePath),
		})
		volume.Name = to.StringPtr(stateMountName)
		volume.AzureFile = d.getAzureFileVolume()
		volumeMount.Name = to.StringPtr(stateMountName)
		volumeMount.ReadOnly = to.BoolPtr(false)
		volumeMount.MountPath = to.StringPtr(d.stateMountPoint)
		mounts = append(mounts, volumeMount)
		volumes = append(volumes, volume)
	}

	identity, err := d.getContainerIdentity(ctx, d.aciRG)
	if err != nil {
		return fmt.Errorf("Failed to get container Identity:%v", err)
	}

	_, err = d.createInstance(d.aciName, d.aciLocation, d.aciRG, image, env, *identity, &mounts, &volumes, hasFiles, domain)
	if err != nil {
		return fmt.Errorf("Error creating ACI Instance:%v", err)
	}

	// TODO: Check if ACR under ACI supports MSI
	// TODO: Login to ACR if the registry is azurecr.io
	// TODO: Add support for private registry

	if d.deleteACIResources {
		defer func() {
			fmt.Println("Cleaning up Azure Resources created to execute Bundle")
			log.Debug("Deleting Container Instance ", d.aciName)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			containerGroupsClient, err := az.GetContainerGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting Container Groups Client: %v\n", err)
			}

			_, err = containerGroupsClient.Delete(ctx, d.aciRG, d.aciName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to delete container error: %v\n", err)
			}

			log.Debug("Deleted Container ", d.aciName)
		}()
	}

	fmt.Println("Running Bundle Instance in Azure Container Instance")
	// Check if the container is running
	state, err := d.getContainerState(d.aciRG, d.aciName)
	if err != nil {
		return fmt.Errorf("Error getting container state :%v", err)
	}

	// Get the logs if the container failed immediately
	if strings.Compare(state, "Failed") == 0 {
		_, err := d.getContainerLogs(ctx, d.aciRG, d.aciName, 0)
		if err != nil {
			return fmt.Errorf("Error getting container logs :%v", err)
		}

		return errors.New("Container execution failed")
	}

	containerRunning := true
	linesOutput := 0
	for containerRunning {
		log.Debug("Getting ACI State")
		state, err := d.getContainerState(d.aciRG, d.aciName)
		if err != nil {
			return fmt.Errorf("Error getting container state :%v", err)
		}

		if strings.Compare(state, "Running") == 0 {
			linesOutput, err = d.getContainerLogs(ctx, d.aciRG, d.aciName, linesOutput)
			if err != nil {
				return fmt.Errorf("Error getting container logs :%v", err)
			}

			log.Debug("Sleeping waiting for Container to complete")
			fmt.Print("\033[1C\033[1D")
			time.Sleep(5 * time.Second)
		} else {
			if strings.Compare(state, "Succeeded") != 0 {
				// Log any error getting container logs
				if _, err = d.getContainerLogs(ctx, d.aciRG, d.aciName, linesOutput); err != nil {
					log.Debugf("Error getting Container Logs: %v", err)
				}
				return fmt.Errorf("Unexpected Container Status:%s", state)
			}

			containerRunning = false
		}

	}

	_, err = d.getContainerLogs(ctx, d.aciRG, d.aciName, linesOutput)
	if err != nil {
		return fmt.Errorf("Error getting container logs :%v", err)
	}

	log.Debug("Container terminated successfully")
	return nil
}

// This function creates an AzureFileVolume to be used by the bundle for state storage
func (d *aciDriver) getAzureFileVolume() *containerinstance.AzureFileVolume {

	azureFileVolume := containerinstance.AzureFileVolume{}
	if d.mountStateVolume {
		azureFileVolume.ReadOnly = to.BoolPtr(false)
		azureFileVolume.StorageAccountKey = to.StringPtr(d.stateStorageAccountKey)
		azureFileVolume.StorageAccountName = to.StringPtr(d.stateStorageAccountName)
		azureFileVolume.ShareName = to.StringPtr(d.stateFileShare)
	}
	return &azureFileVolume
}

// This will only work if the logs don't get truncated because of size.
func (d *aciDriver) getContainerLogs(ctx context.Context, aciRG string, aciName string, linesOutput int) (int, error) {
	log.Debug("Getting Logs from Invocation Image")
	containerClient, err := az.GetContainerClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	if err != nil {
		return 0, fmt.Errorf("Error getting Container Client: %v", err)
	}

	logs, err := containerClient.ListLogs(ctx, aciRG, aciName, aciName, nil)
	if err != nil {
		return 0, fmt.Errorf("Error getting container logs :%v", err)
	}

	lines := strings.Split(strings.TrimSuffix(*logs.Content, "\n"), "\n")
	noOfLines := len(lines)
	for currentLine := linesOutput; currentLine < noOfLines; currentLine++ {
		_, err := fmt.Println(lines[currentLine])
		if err != nil {
			return 0, fmt.Errorf("Error writing container logs :%v", err)
		}

	}

	return noOfLines, nil
}

func (d *aciDriver) getContainerIdentity(ctx context.Context, aciRG string) (*identityDetails, error) {

	// System MSI
	if d.msiType == "system" {

		//TODO Validate Role and Scope
		//TODO Check to see if user has permission to create RoleAssignment

		if len(d.systemMSIScope) == 0 {
			d.systemMSIScope = fmt.Sprintf("/subscriptions/%s/resourcegroups/%s", d.subscriptionID, d.aciRG)
			log.Debugf("Set system MSI Scope to %s", d.systemMSIScope)
		}

		return &identityDetails{
			MSIType: "system",
			Identity: &containerinstance.ContainerGroupIdentity{
				Type: containerinstance.SystemAssigned,
			},
			Scope: &d.systemMSIScope,
			Role:  &d.systemMSIRole,
		}, nil
	}

	// User MSI
	if d.msiType == "user" {
		userAssignedIdentitiesClient, err := az.GetUserAssignedIdentitiesClient(d.msiResource.SubscriptionID, d.loginInfo.Authorizer, d.userAgent)
		if err != nil {
			return nil, fmt.Errorf("Error getting User Assigned Identities Client: %v", err)
		}

		identity, err := userAssignedIdentitiesClient.Get(ctx, d.msiResource.ResourceGroup, d.msiResource.ResourceName)
		if err != nil {
			return nil, fmt.Errorf("Error getting User Assigned Identity:%v  Error: %v", d.msiResource, err)
		}

		// Check if permission to use MSI

		if az.IsInCloudShell() {
			scope := fmt.Sprintf("subscriptions/%s/resourceGroups/%s/providers/%s/%s/%s", d.msiResource.SubscriptionID, d.msiResource.ResourceGroup, d.msiResource.Provider, d.msiResource.ResourceType, d.msiResource.ResourceName)
			log.Debug("Checking permission to assign MSI at scope: ", scope)
			if canAssignMSI, err := az.CheckCanAccessResource("Microsoft.ManagedIdentity/userAssignedIdentities/assign/action", scope); err != nil || !canAssignMSI {
				// TODO This check is producing false negatives (user with access is getting Not Allowed just log response for now)
				if err != nil {
					log.Debug(fmt.Sprintf("Failed checking access for MSI Assign at scope %s Error: %v", scope, err))
					//return nil, fmt.Errorf("Failed checking access for MSI Assign at scope %s Error: %v", scope, err)
				}
				log.Debug(fmt.Sprintf("You do not have permission to assign MSI %s", scope))
				//return nil, fmt.Errorf("You do not have permission to assign MSI %s", scope)
			}
		}

		return &identityDetails{
			MSIType: "user",
			Identity: &containerinstance.ContainerGroupIdentity{
				Type: containerinstance.UserAssigned,
				UserAssignedIdentities: map[string]*containerinstance.ContainerGroupIdentityUserAssignedIdentitiesValue{
					*identity.ID: {},
				},
			},
		}, nil
	}

	return &identityDetails{
		MSIType: "none",
	}, nil

}

func (d *aciDriver) getContainerState(aciRG string, aciName string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	containerGroupsClient, err := az.GetContainerGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	if err != nil {
		return "", fmt.Errorf("Error getting Container Groups Client: %v", err)
	}

	resp, err := containerGroupsClient.Get(ctx, aciRG, aciName)
	if err != nil {
		return "", err
	}

	return *resp.InstanceView.State, nil
}

func locationIsAvailable(location string, locations []string) bool {
	for _, l := range locations {
		l = strings.ToLower(strings.Replace(l, " ", "", -1))
		if l == location {
			return true
		}

	}
	return false
}

func (d *aciDriver) createContainerGroup(aciName string, aciRG string, containerGroup containerinstance.ContainerGroup) (containerinstance.ContainerGroup, error) {
	containerGroupsClient, err := az.GetContainerGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	if err != nil {
		return containerinstance.ContainerGroup{}, fmt.Errorf("Error getting Container Groups Client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	future, err := containerGroupsClient.CreateOrUpdate(ctx, aciRG, aciName, containerGroup)
	if err != nil {
		return containerinstance.ContainerGroup{}, fmt.Errorf("Error Creating Container Group: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, containerGroupsClient.Client)
	if err != nil {
		return containerinstance.ContainerGroup{}, fmt.Errorf("Error Waiting for Container Group creation: %v", err)
	}

	return future.Result(*containerGroupsClient)
}

func (d *aciDriver) createInstance(aciName string, aciLocation string, aciRG string, image string, env []containerinstance.EnvironmentVariable, identity identityDetails, mounts *[]containerinstance.VolumeMount, volumes *[]containerinstance.Volume, hasFiles bool, domain string) (*containerinstance.ContainerGroup, error) {

	// TODO Windows Container support

	// ARM does not yet support the ability to create a System MSI and assign role and scope on creation
	// so if the MSI type is system assigned then need to create the ACI Instance first with an alpine instance in order to create the identity and then assign permissions
	// The created ACI is then updated to execute the Invocation Image

	if identity.MSIType == "system" {
		log.Debug("Creating ACI to create System Identity")
		alpine := "alpine:latest"
		containerGroup, err := d.createContainerGroup(
			aciName,
			aciRG,
			containerinstance.ContainerGroup{
				Name:     &aciName,
				Location: &aciLocation,
				Identity: identity.Identity,
				ContainerGroupProperties: &containerinstance.ContainerGroupProperties{
					OsType:        containerinstance.Linux,
					RestartPolicy: containerinstance.Never,
					Containers: &[]containerinstance.Container{
						{
							Name: &aciName,
							ContainerProperties: &containerinstance.ContainerProperties{
								Image: &alpine,
								Resources: &containerinstance.ResourceRequirements{
									Requests: &containerinstance.ResourceRequests{
										MemoryInGB: to.Float64Ptr(1),
										CPU:        to.Float64Ptr(1.5),
									},
									Limits: &containerinstance.ResourceLimits{
										MemoryInGB: to.Float64Ptr(1),
										CPU:        to.Float64Ptr(1.5),
									},
								},
							},
						},
					},
				},
			})
		if err != nil {
			return nil, fmt.Errorf("Error Creating Container Group for System MSI creation: %v", err)
		}

		err = d.setUpSystemMSIRBAC(containerGroup.Identity.PrincipalID, *identity.Scope, *identity.Role)
		if err != nil {
			return nil, fmt.Errorf("Error setting up RBAC for System MSI : %v", err)
		}

	}

	log.Debug("Creating ACI for CNAB action")

	// Because ACI does not have a way to mount or copy files any file input to the invocation image is set as a pair of secrets in a secret volume, path{n} contains the file target file path and
	// value{n} contains the file content, the script below is injected into the container so that the expected files are created before the run tool is executed
	var scriptBuilder strings.Builder
	var command []string

	if hasFiles || d.hasOutputs || d.debugContainer {
		if hasFiles {
			// Get the filenames and data  from the secret volume and place them where they are expected by the bundle
			scriptBuilder.WriteString(fmt.Sprintf("cd %s;for f in $(ls path*);do file=$(cat ${f});mkdir -p $(dirname ${file});cp value${f#path} ${file};done;cd -;", fileMountPoint))
		}

		if len(d.statePath) > 0 {
			statePathCmd := "mkdir -p ${STATE_PATH};"
			scriptBuilder.WriteString(statePathCmd)
		}

		if d.hasOutputs {
			outputsCmd := fmt.Sprintf("mkdir -p ${STATE_PATH}/%[2]s;ln -s ${STATE_PATH}/%[2]s %[1]s%[2]s;", cnabOutputMountPoint, cnabOutputDirName)
			scriptBuilder.WriteString(outputsCmd)
		}
		// This is to allow attaching to the container for debug purposes
		if d.debugContainer {
			scriptBuilder.WriteString("tail -f /dev/null")
		} else {
			scriptBuilder.WriteString("/cnab/app/run")
		}

		command = []string{"/bin/bash", "-e", "-c", scriptBuilder.String()}
	}

	var registrycredentials []containerinstance.ImageRegistryCredential

	if len(d.imageRegistryPassword) > 0 {
		credentials := containerinstance.ImageRegistryCredential{
			Username: &d.imageRegistryUser,
			Password: &d.imageRegistryPassword,
			Server:   &domain,
		}
		registrycredentials = append(registrycredentials, credentials)
	}

	containerGroup, err := d.createContainerGroup(
		aciName,
		aciRG,
		containerinstance.ContainerGroup{
			Name:     &aciName,
			Location: &aciLocation,
			Identity: identity.Identity,
			ContainerGroupProperties: &containerinstance.ContainerGroupProperties{
				OsType:        containerinstance.Linux,
				RestartPolicy: containerinstance.Never,
				Containers: &[]containerinstance.Container{
					{
						Name: &aciName,
						ContainerProperties: &containerinstance.ContainerProperties{
							Image: &image,
							Resources: &containerinstance.ResourceRequirements{
								Requests: &containerinstance.ResourceRequests{
									MemoryInGB: to.Float64Ptr(1),
									CPU:        to.Float64Ptr(1.5),
								},
								Limits: &containerinstance.ResourceLimits{
									MemoryInGB: to.Float64Ptr(1),
									CPU:        to.Float64Ptr(1.5),
								},
							},
							EnvironmentVariables: &env,
							Command:              to.StringSlicePtr(command),
							VolumeMounts:         mounts,
						},
					},
				},
				Volumes:                  volumes,
				ImageRegistryCredentials: &registrycredentials,
			},
		})
	if err != nil {
		return nil, fmt.Errorf("Error Creating Container Group: %v", err)
	}

	return &containerGroup, nil
}

func (d *aciDriver) setUpSystemMSIRBAC(principalID *string, scope string, role string) error {
	log.Debug("Setting up System MSI Scope ", scope, "Role ", role)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	roleDefinitionsClient, err := az.GetRoleDefinitionsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	if err != nil {
		return fmt.Errorf("Error getting RoleDefinitions Client: %v", err)
	}
	roleDefinitionID := ""
	for roleDefinitions, err := roleDefinitionsClient.ListComplete(ctx, scope, ""); roleDefinitions.NotDone(); err = roleDefinitions.NextWithContext(ctx) {
		if err != nil {
			return fmt.Errorf("Error getting RoleDefinitions for Scope:%s Error: %v", scope, err)
		}

		if *roleDefinitions.Value().RoleDefinitionProperties.RoleName == role {
			roleDefinitionID = *roleDefinitions.Value().ID
			break
		}

	}

	if roleDefinitionID == "" {
		return fmt.Errorf("Role Definition for Role %s not found for Scope:%s", role, scope)
	}

	log.Debug("Role Definition Id: ", roleDefinitionID)
	// Wait for principal to be available
	attempts := 5
	for i := 0; i < attempts; i++ {
		log.Debug("Creating RoleAssignment Attempt: ", i)
		roleAssignmentsClient, raerror := az.GetRoleAssignmentClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
		if raerror != nil {
			log.Debug("Failed to Get Role Assignment Client Error: ", err)
		}

		_, raerror = roleAssignmentsClient.Create(ctx, scope, uuid.New().String(), authorization.RoleAssignmentCreateParameters{
			Properties: &authorization.RoleAssignmentProperties{
				RoleDefinitionID: &roleDefinitionID,
				PrincipalID:      principalID,
			},
		})
		if raerror != nil {
			err = fmt.Errorf("Error creating RoleAssignment Role:%s for Scope:%s Error: %v", role, scope, raerror)
			log.Debug("Creating RoleAssignment Attempt: ", i, "Error: ", err)
			time.Sleep(20 * time.Second)
			continue
		}

		err = raerror
		break
	}

	return err
}

type identityDetails struct {
	MSIType  string
	Identity *containerinstance.ContainerGroupIdentity
	Scope    *string
	Role     *string
}

func (d *aciDriver) createMSIEnvVars(env []containerinstance.EnvironmentVariable) []containerinstance.EnvironmentVariable {

	if len(d.msiType) > 0 {
		name := "AZURE_MSI_TYPE"
		env = append(env, containerinstance.EnvironmentVariable{
			Name:        &name,
			SecureValue: &d.msiType,
		})
		log.Debug("Setting Container Group Environment Variable: Name:", name, "Value:", d.msiType)
		if d.msiType == "user" {
			name := "AZURE_USER_MSI_RESOURCE_ID"
			env = append(env, containerinstance.EnvironmentVariable{
				Name:        &name,
				SecureValue: &d.userMSIResourceID,
			})
			log.Debug("Setting Container Group Environment Variable: Name:", name, "Value:", d.userMSIResourceID)
		}
	}

	return env
}

func (d *aciDriver) createAzureEnvironmentEnvVars(env []containerinstance.EnvironmentVariable) []containerinstance.EnvironmentVariable {

	azureEnvironmentPropertyNames := map[string]string{
		"subscriptionID": "AZURE_SUBSCRIPTION_ID",
		"tenantID":       "AZURE_TENANT_ID",
	}
	for k, v := range azureEnvironmentPropertyNames {
		value := d.getFieldValue(k)
		if len(value) > 0 {
			name := v
			env = append(env, containerinstance.EnvironmentVariable{
				Name:        &name,
				SecureValue: &value,
			})
			log.Debug("Setting Container Group Environment Variable: Name:", v, "Value:", value)
		}
	}
	return env
}

func (d *aciDriver) createCredentialEnvVars(env []containerinstance.EnvironmentVariable) ([]containerinstance.EnvironmentVariable, error) {

	name := "AZURE_OAUTH_TOKEN"
	var token string
	if d.loginInfo.LoginType == az.CloudShell || d.loginInfo.LoginType == az.DeviceCode {
		log.Debugf("Propagating OAuth Token from %v login", d.loginInfo.LoginType)
		token = d.loginInfo.OAuthTokenProvider.OAuthToken()
	}

	if d.loginInfo.LoginType == az.CLI {
		log.Debug("Propagating OAuth Token from cli")
		t, err := cli.GetTokenFromCLI("https://management.azure.com/")
		if err != nil {
			return nil, err
		}

		adaltoken, err := t.ToADALToken()
		token = adaltoken.OAuthToken()
		if err != nil {
			return nil, fmt.Errorf("Failed to get cli token for propagation: %v", err)
		}

	}

	env = append(env, containerinstance.EnvironmentVariable{
		Name:        &name,
		SecureValue: &token,
	})
	log.Debug("Setting Container Group Environment Variable: Name: ", name)

	spnPropertyNames := map[string]string{
		"clientID":     "AZURE_CLIENT_ID",
		"clientSecret": "AZURE_CLIENT_SECRET",
	}
	for k, v := range spnPropertyNames {
		value := d.getFieldValue(k)
		if len(value) > 0 {
			name := v
			env = append(env, containerinstance.EnvironmentVariable{
				Name:        &name,
				SecureValue: &value,
			})
			log.Debug("Setting Container Group Environment Variable: Name: ", v)
		}
	}
	return env, nil
}

func (d *aciDriver) getFieldValue(field string) string {
	r := reflect.ValueOf(d)
	return reflect.Indirect(r).FieldByName(field).String()
}

func imageWithDigest(img bundle.InvocationImage) string {
	// its not clear what should be in img.Image and img.Digest this tries to cover multiple scenarios
	// see https://github.com/deislabs/cnab-go/issues/145 and https://github.com/deislabs/cnab-spec/issues/287
	log.Debug("Image: ", img.Image)
	// Hack in case image contains digest and/or digest and tag
	if img.Digest == "" || strings.Contains(img.Image, "@sha256") {
		imageParts := strings.Split(img.Image, ":")
		// Image contains a tag and a digest
		if len(imageParts) == 3 {
			return imageParts[0] + ":" + imageParts[2]
		}
		return img.Image
	}

	//Workaround for issue in ACI Image Reference Parsing where it doesn't allow both a tag and a digest
	return strings.Split(img.Image, ":")[0] + "@" + img.Digest
}
