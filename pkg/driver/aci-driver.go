package driver

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/deislabs/cnab-go/driver"
	"github.com/docker/distribution/reference"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	az "github.com/deislabs/duffle-aci-driver/pkg/azure"

	"os"
	"strings"
	"time"
)

const (
	userAgentPrefix = "DuffleACIDriver"
	fileMountPoint  = "/mnt/BundleFiles"
	fileMountName   = "bundlefilevolume"
	stateMountName  = "state"
)

// aciDriver runs Docker and OCI invocation images in ACI
type aciDriver struct {
	//config                map[string]string
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
	mountStateVolume        bool
	stateFileShare          string
	stateStorageAccountName string
	stateStorageAccountKey  string
	statePath               string
	stateMountPoint         string
	userAgent               string
	loginInfo               az.LoginInfo
}

// Config returns the ACI driver configuration options
func (d *aciDriver) Config() map[string]string {
	return map[string]string{
		"DUFFLE_ACI_DRIVER_VERBOSE":                            "Increase verbosity. true, false are supported values",
		"DUFFLE_ACI_DRIVER_CLIENT_ID":                          "AAD Client ID for Azure account authentication - used to authenticate to Azure for ACI creation",
		"DUFFLE_ACI_DRIVER_CLIENT_SECRET":                      "AAD Client Secret for Azure account authentication - used to authenticate to Azure for ACI creation",
		"DUFFLE_ACI_DRIVER_TENANT_ID":                          "Azure AAD Tenant Id Azure account authentication - used to authenticate to Azure for ACI creation",
		"DUFFLE_ACI_DRIVER_SUBSCRIPTION_ID":                    "Azure Subscription Id - this is the subscription to be used for ACI creation, if not specified the default subscription is used",
		"DUFFLE_ACI_DRIVER_APP_ID":                             "Azure Application Id - this is the application to be used to authenticate to Azure",
		"DUFFLE_ACI_DRIVER_RESOURCE_GROUP":                     "The name of the existing Resource Group to create the ACI instance in, if not specified a Resource Group will be created",
		"DUFFLE_ACI_DRIVER_LOCATION":                           "The location to create the ACI Instance in",
		"DUFFLE_ACI_DRIVER_NAME":                               "The name of the ACI instance to create - if not specified a name will be generated",
		"DUFFLE_ACI_DRIVER_DELETE_RESOURCES":                   "Delete RG and ACI instance created - default is true useful to set to false for debugging - only deletes RG if it was created by the driver",
		"DUFFLE_ACI_DRIVER_MSI_TYPE":                           "This can be set to user or system",
		"DUFFLE_ACI_DRIVER_SYSTEM_MSI_ROLE":                    "The role to be asssigned to System MSI User - used if DUFFLE_ACI_DRIVER_ACI_MSI_TYPE == system, if this is null or empty then the role defaults to contributor",
		"DUFFLE_ACI_DRIVER_SYSTEM_MSI_SCOPE":                   "The scope to apply the role to System MSI User - will attempt to set scope to the  Resource Group that the ACI Instance is being created in if not set",
		"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID":               "The resource Id of the MSI User - required if DUFFLE_ACI_DRIVER_ACI_MSI_TYPE == User ",
		"DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS":              "If this is set to true the credentials used to Launch the Driver are propagated to the invocation image in an ENV variable DUFFLE_ACI_DRIVER prefix will be relaced with AZURE_, default is false",
		"DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH": "If this is set to true the DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET are also used for authentication to ACR",
		"DUFFLE_ACI_DRIVER_REGISTRY_USERNAME":                  "The username for authenticating to the container registry",
		"DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD":                  "The password for authenticating to the container registry",
		"DUFFLE_ACI_DRIVER_STATE_FILESHARE":                    "The File Share for Azure State volume",
		"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME":         "The Storage Account for the Azure State File Share",
		"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY":          "The Storage Key for the Azure State File Share",
		"DUFFLE_ACI_DRIVER_STATE_PATH":                         "The local path relative to the mount point where state can be stored - this is set as a environment variable on the ACI instance and can be used by a bundle to persist filesystem data",
		"DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT":                  "The mount point location for state volume",
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
	if len(config["DUFFLE_ACI_DRIVER_DELETE_RESOURCES"]) > 0 && (strings.ToLower(config["DUFFLE_ACI_DRIVER_DELETE_RESOURCES"]) == "false") {
		d.deleteACIResources = false
	}
	log.Debug("Delete Resources:", d.deleteACIResources)

	// Azure AAD Client Id for authenticating to Azure
	d.clientID = config["DUFFLE_ACI_DRIVER_CLIENT_ID"]
	log.Debug("clientID:", d.clientID)

	// Azure AAD Client Secret for authenticating to Azure
	d.clientSecret = config["DUFFLE_ACI_DRIVER_CLIENT_SECRET"]
	log.Debug("clientSecret:", len(d.clientSecret) > 0)

	//Validate that both of Client Id, CLient Secret and Tenant Id are set
	clientCreds, err := checkAllOrNoneSet(config, []string{"DUFFLE_ACI_DRIVER_CLIENT_ID", "DUFFLE_ACI_DRIVER_CLIENT_SECRET"})
	if err != nil {
		return err
	}

	// Azure Tenant Id for authenticating to Azure
	d.tenantID = config["DUFFLE_ACI_DRIVER_TENANT_ID"]
	log.Debug("tenantID:", d.tenantID)

	// Azure Application Id to be used with device code auth flow
	d.applicationID = config["DUFFLE_ACI_DRIVER_APP_ID"]
	log.Debug("applicationID:", d.applicationID)
	appID := len(d.applicationID) > 0

	// SPN and appId are mutually exclusive
	if clientCreds && appID {
		return errors.New("either DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID should be set not both")
	}

	// TenantId is required when client credentials or DUFFLE_ACI_DRIVER_APP_ID is set
	if (clientCreds || appID) && len(d.tenantID) == 0 {
		return errors.New("DUFFLE_ACI_DRIVER_TENANT_ID should be set when DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID are set")
	}

	// TenantId should not be set if client creds or app id not set
	if !clientCreds && !appID && len(d.tenantID) > 0 {
		return errors.New("DUFFLE_ACI_DRIVER_TENANT_ID should not be set when DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID are not set")
	}

	// TODO check for default subscription in azure CLI config
	// Azure Subscription Id to create resources to run invocation image in - if this is not set then the first subscription found will be used
	d.subscriptionID = config["DUFFLE_ACI_DRIVER_SUBSCRIPTION_ID"]
	log.Debug("Subscription:", d.subscriptionID)

	// Check to see if an resource group name has been set, if not then a location must be set , if an resource group name is set and no location is used then the resource group must already exist and the location of he resource group will be used for the resources
	d.aciRG = config["DUFFLE_ACI_DRIVER_RESOURCE_GROUP"]
	log.Debug("Resource Group:", d.aciRG)
	d.aciLocation = strings.ToLower(strings.Replace(config["DUFFLE_ACI_DRIVER_LOCATION"], " ", "", -1))
	log.Debug("Location:", d.aciLocation)
	if len(d.aciRG) == 0 && len(d.aciLocation) == 0 {
		// TODO check if running in cloudshell or cli configured and defaults are set
		return errors.New("ACI Driver requires DUFFLE_ACI_DRIVER_LOCATION environment variable or an existing Resource Group in DUFFLE_ACI_DRIVER_RESOURCE_GROUP")
	}

	// If Resource group name is not set generate a unique name
	d.createRG = len(d.aciRG) == 0
	if d.createRG {
		d.aciRG = uuid.New().String()
		log.Debug("New Resource Group Name:", d.aciRG)
	}

	// If aci driver name is not set generate a unique aci name
	d.aciName = config["DUFFLE_ACI_DRIVER_NAME"]
	if len(d.aciName) == 0 {
		d.aciName = fmt.Sprintf("duffle-%s", uuid.New().String())
	}

	log.Debug("Generated ACI Name:", d.aciName)
	// Check that if MSI type is set it is either user or system, if it is user then there must also be a valid resource id set for the user MSI
	if len(config["DUFFLE_ACI_DRIVER_MSI_TYPE"]) > 0 {
		log.Debug("MSI Type", config["DUFFLE_ACI_DRIVER_MSI_TYPE"])
		switch strings.ToLower(config["DUFFLE_ACI_DRIVER_MSI_TYPE"]) {
		case "system":
			d.msiType = "system"
			d.systemMSIRole = "Contributor"
			if len(config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_ROLE"]) > 0 {
				d.systemMSIRole = config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_ROLE"]
			}
			log.Debug("System MSI Role:", d.systemMSIRole)

			d.systemMSIScope = ""
			if len(config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_SCOPE"]) > 0 {
				d.systemMSIScope = config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_SCOPE"]
				_, err := azure.ParseResourceID(d.systemMSIScope)
				if err != nil {
					return fmt.Errorf("DUFFLE_ACI_DRIVER_SYSTEM_MSI_SCOPE environment variable parsing error: %v", err)
				}
				log.Debugf("System MSI Scope %s:", d.systemMSIScope)
			} else {
				log.Debugf("System MSI Scope Not Set will default to RG %s Scope:", d.aciRG)
			}

		case "user":
			d.msiType = "user"
			d.userMSIResourceID = config["DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID"]
			log.Debug("User MSI Resource ID:", d.userMSIResourceID)

			if len(d.userMSIResourceID) == 0 {
				return errors.New("ACI Driver requires DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable when DUFFLE_ACI_DRIVER_MSI_TYPE is set to user")
			}

			resource, err := azure.ParseResourceID(d.userMSIResourceID)
			if err != nil {
				return fmt.Errorf("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable parsing error: %v", err)
			}

			if strings.ToLower(resource.Provider) != "microsoft.managedidentity" || strings.ToLower(resource.ResourceType) != "userassignedidentities" {
				return fmt.Errorf("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: %s/%s", resource.Provider, resource.ResourceType)
			}

			d.msiResource = resource

		default:
			return fmt.Errorf("DUFFLE_ACI_DRIVER_MSI_TYPE environment variable unknown value: %s", config["DUFFLE_ACI_DRIVER_MSI_TYPE"])
		}
	}

	// Propagation of Credentials enables the flow of Azure credentials from the ACI_DRIVER to the invocation image
	d.propagateCredentials = len(config["DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS"]) > 0 && strings.ToLower(config["DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS"]) == "true"
	log.Debug("Propagate Credentials:", d.propagateCredentials)

	// Credentials to be used for container registry for the invocation image
	d.imageRegistryUser = config["DUFFLE_ACI_DRIVER_REGISTRY_USERNAME"]
	d.imageRegistryPassword = config["DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD"]

	// Both DUFFLE_ACI_DRIVER_REGISTRY_USERNAME and DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD are required
	registryCredsSet, err := checkAllOrNoneSet(config, []string{"DUFFLE_ACI_DRIVER_REGISTRY_USERNAME", "DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD"})
	if err != nil {
		return err
	}

	// DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH enables the SPN to also be used for authenticating to the registry that contains the invocation image
	d.useSPForACR = len(config["DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH"]) > 0 && strings.ToLower(config["DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH"]) == "true"
	if d.useSPForACR {
		if registryCredsSet {
			return errors.New("DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH should not be set if DUFFLE_ACI_DRIVER_REGISTRY_USERNAME and DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD are set")
		}
		if len(d.clientID) == 0 || len(d.clientSecret) == 0 {
			return errors.New("Both DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET should be set when setting DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH")
		}
		d.imageRegistryPassword = d.clientSecret
		d.imageRegistryUser = d.clientID
	}

	// DUFFLE_ACI_DRIVER_STATE_* allows an Azure File Share to be mounted to the invocation image sto be used for instance state
	d.mountStateVolume, err = checkAllOrNoneSet(config, []string{"DUFFLE_ACI_DRIVER_STATE_PATH", "DUFFLE_ACI_DRIVER_STATE_FILESHARE", "DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME", "DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY", "DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT"})
	if err != nil {
		return err
	}

	if d.mountStateVolume {
		// TODO Allow empty storage account key and do runtime lookup
		if !path.IsAbs(config["DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT"]) {
			return fmt.Errorf("value (%s) of DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT is not an absolute path", config["DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT"])
		}

		d.stateFileShare = config["DUFFLE_ACI_DRIVER_STATE_FILESHARE"]
		d.stateMountPoint = config["DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT"]
		d.statePath = config["DUFFLE_ACI_DRIVER_STATE_PATH"]
		d.stateStorageAccountName = config["DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME"]
		d.stateStorageAccountKey = config["DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY"]
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
func (d *aciDriver) Run(op *driver.Operation) error {
	return d.exec(op)
}

// Handles indicates that the ACI driver supports "docker" and "oci"
func (d *aciDriver) Handles(dt string) bool {
	return dt == driver.ImageTypeDocker || dt == driver.ImageTypeOCI
}

func (d *aciDriver) exec(op *driver.Operation) (reterr error) {
	var err error
	d.loginInfo, err = az.LoginToAzure(d.clientID, d.clientSecret, d.tenantID, d.applicationID)
	if err != nil {
		return fmt.Errorf("cannot Login To Azure: %v", err)
	}

	err = d.setAzureSubscriptionID()
	if err != nil {
		return fmt.Errorf("cannot set Azure subscription: %v", err)
	}

	err = d.runInvocationImageUsingACI(op)
	if err != nil {
		return fmt.Errorf("running invocation instance using ACI failed: %v", err)
	}

	return nil
}

func (d *aciDriver) setAzureSubscriptionID() error {

	subscriptionsClient := az.GetSubscriptionsClient(d.loginInfo.Authorizer, d.userAgent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if len(d.subscriptionID) != 0 {
		log.Debug("Checking for Subscription ID:", d.subscriptionID)
		result, err := subscriptionsClient.Get(ctx, d.subscriptionID)
		if err != nil {
			if result.StatusCode == 404 {
				return fmt.Errorf("Subscription Id: %s not found", d.subscriptionID)
			}

			return fmt.Errorf("Attempt to Get Subscription Failed: %v", err)
		}

	} else {
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
			log.Debug("Setting Subscription to", subscriptionID)
			d.subscriptionID = subscriptionID
		} else {
			return errors.New("Cannot find a subscription")
		}

	}

	return nil
}

func (d *aciDriver) runInvocationImageUsingACI(op *driver.Operation) error {

	// TODO Check that image is a type and platform that can be executed by ACI

	// GET ACI Config

	ref, err := reference.ParseAnyReference(op.Image)
	if err != nil {
		return fmt.Errorf("Failed to parse image reference: %s", op.Image)
	}

	var domain string
	if named, ok := ref.(reference.Named); ok {
		domain = reference.Domain(named)
	}

	if d.useSPForACR && !strings.HasSuffix(domain, "azurecr.io") {
		return fmt.Errorf("Cannot use Service Principal as credentials for non Azure registry : %s", domain)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	groupsClient := az.GetGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	if !d.createRG {
		rg, err := groupsClient.Get(ctx, d.aciRG)
		if err != nil {
			return fmt.Errorf("Checking for existing resource group %s failed with error: %v", d.aciRG, err)
		}

		if len(d.aciLocation) == 0 {
			log.Debug("Setting aci Location to RG Location:", *rg.Location)
			d.aciLocation = *rg.Location
		}

	}

	// Check that location supports ACI

	providersClient := az.GetProvidersClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
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
		log.Debug("Creating Resource Group")
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
				log.Debug("Deleting Resource Group")
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

	var mounts []containerinstance.VolumeMount
	var volumes []containerinstance.Volume

	// ACI does not support file copy
	// files are mounted into the container in a secrets volume and invocationImage Entry point is modified to process the files before run cmd is invoked

	hasFiles := false
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
			log.Debug("File", k, "Value", v)
			secrets[fmt.Sprintf("path%d", i)] = to.StringPtr(base64.StdEncoding.EncodeToString([]byte(k)))
			secrets[fmt.Sprintf("value%d", i)] = to.StringPtr(base64.StdEncoding.EncodeToString([]byte(v)))
			i++
		}
	}

	log.Debug("Bundle Has Files:", hasFiles)

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
				log.Debug("Updating Container Group Environment Variable: Name:", k, "to Value:", v)
				continue
			}

		}
		env = append(env, containerinstance.EnvironmentVariable{
			Name:        to.StringPtr(k),
			SecureValue: to.StringPtr(strings.Replace(v, "'", "''", -1)),
		})
		log.Debug("Setting Container Group Environment Variable: Name:", k, "Value:", v)
	}
	var volume = containerinstance.Volume{}
	var volumeMount = containerinstance.VolumeMount{}
	if d.mountStateVolume {
		statePath := fmt.Sprintf("%s/%s", d.stateMountPoint, d.statePath)
		env = append(env, containerinstance.EnvironmentVariable{
			Name:        to.StringPtr("STATE_PATH"),
			SecureValue: to.StringPtr(statePath),
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

	_, err = d.createInstance(d.aciName, d.aciLocation, d.aciRG, op.Image, env, *identity, &mounts, &volumes, hasFiles, domain)
	if err != nil {
		return fmt.Errorf("Error creating ACI Instance:%v", err)
	}

	// TODO: Check if ACR under ACI supports MSI
	// TODO: Login to ACR if the registry is azurecr.io
	// TODO: Add support for private registry

	if d.deleteACIResources {
		defer func() {
			log.Debug("Deleting Container Instance")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			containerGroupsClient := az.GetContainerGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
			_, err := containerGroupsClient.Delete(ctx, d.aciRG, d.aciName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to delete container error: %v\n", err)
			}

			log.Debug("Deleted Container ", d.aciName)
		}()
	}

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
		log.Debug("Getting Container State")
		state, err := d.getContainerState(d.aciRG, d.aciName)
		if err != nil {
			return fmt.Errorf("Error getting container state :%v", err)
		}

		if strings.Compare(state, "Running") == 0 {
			linesOutput, err = d.getContainerLogs(ctx, d.aciRG, d.aciName, linesOutput)
			if err != nil {
				return fmt.Errorf("Error getting container logs :%v", err)
			}

			log.Debug("Sleeping")
			fmt.Print("\033[1C\033[1D")
			time.Sleep(5 * time.Second)
		} else {
			if strings.Compare(state, "Succeeded") != 0 {
				d.getContainerLogs(ctx, d.aciRG, d.aciName, linesOutput)
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
	log.Debug("Getting Invocation Image Logs")
	containerClient := az.GetContainerClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
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
		userAssignedIdentitiesClient := az.GetUserAssignedIdentitiesClient(d.msiResource.SubscriptionID, d.loginInfo.Authorizer, d.userAgent)
		identity, err := userAssignedIdentitiesClient.Get(ctx, d.msiResource.ResourceGroup, d.msiResource.ResourceName)
		if err != nil {
			return nil, fmt.Errorf("Error getting User Assigned Identity:%v  Error: %v", d.msiResource, err)
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
	containerGroupsClient := az.GetContainerGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
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
	containerGroupsClient := az.GetContainerGroupsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
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

	return future.Result(containerGroupsClient)
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

	var command []string
	if hasFiles {
		command = []string{"sh", "-c", fmt.Sprintf("cd %s;for f in $(ls path*);do v=$(cat value${f#path});file=$(cat ${f});mkdir -p $(dirname ${file});echo ${v} > ${file};done;/cnab/app/run", fileMountPoint)}
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
	roleDefinitionsClient := az.GetRoleDefinitionsClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
	roleDefinitionID := ""
	for roleDefinitions, err := roleDefinitionsClient.ListComplete(ctx, scope, ""); roleDefinitions.NotDone(); err = roleDefinitions.NextWithContext(ctx) {
		if err != nil {
			return fmt.Errorf("Error getting RoleDefinitions for Scope:%s Error: %v", scope, err)
		}

		if *roleDefinitions.Value().Properties.RoleName == role {
			roleDefinitionID = *roleDefinitions.Value().ID
			break
		}

	}

	if roleDefinitionID == "" {
		return fmt.Errorf("Role Definition for Role %s not found for Scope:%s", role, scope)
	}

	log.Debug("RoleDefinitionId", roleDefinitionID)
	// Wait for principal to be available
	attempts := 5
	var err error
	for i := 0; i < attempts; i++ {
		log.Debug("Creating RoleAssignment Attempt", i)
		roleAssignmentsClient := az.GetRoleAssignmentClient(d.subscriptionID, d.loginInfo.Authorizer, d.userAgent)
		_, raerror := roleAssignmentsClient.Create(ctx, scope, uuid.New().String(), authorization.RoleAssignmentCreateParameters{
			Properties: &authorization.RoleAssignmentProperties{
				RoleDefinitionID: &roleDefinitionID,
				PrincipalID:      principalID,
			},
		})
		if raerror != nil {
			err = fmt.Errorf("Error creating RoleAssignment Role:%s for Scope:%s Error: %v", role, scope, raerror)
			log.Debug("Creating RoleAssignment Attempt:", i, "Error:", err)
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
		log.Debug(fmt.Sprintf("Propagating OAuth Token from %v login", d.loginInfo.LoginType))
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
	log.Debug("Setting Container Group Environment Variable: Name:", name, "Value:", token)

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
			log.Debug("Setting Container Group Environment Variable: Name:", v, "Value:", value)
		}
	}
	return env, nil
}

func (d *aciDriver) getFieldValue(field string) string {
	r := reflect.ValueOf(d)
	return reflect.Indirect(r).FieldByName(field).String()
}
