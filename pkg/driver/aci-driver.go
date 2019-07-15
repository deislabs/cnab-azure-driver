package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2015-11-01/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/deislabs/cnab-go/driver"
	"github.com/google/uuid"

	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const userAgent string = "Duffle ACI Driver"
const msiTokenEndpoint = "http://169.254.169.254/metadata/identity/oauth2/token"

// aciDriver runs Docker and OCI invocation images in ACI
type aciDriver struct {
	config map[string]string
	// This property is set to true if Duffle is running in cloud shell
	inCloudShell         bool
	deleteACIResources   bool
	authorizer           autorest.Authorizer
	subscriptionID       string
	verbose              bool
	clientID             string
	clientSecret         string
	tenantID             string
	applicationID        string
	aciRG                string
	createRG             bool
	aciLocation          string
	aciName              string
	msiType              string
	msiResource          azure.Resource
	systemMSIScope       string
	systemMSIRole        string
	propagateCredentials bool
	userMSIResourceID    string
	clientLogin          string
	oauthTokenProvider   adal.OAuthTokenProvider
}

// Config returns the ACI driver configuration options
func (d *aciDriver) Config() map[string]string {
	return map[string]string{
		"DUFFLE_ACI_DRIVER_VERBOSE":               "Increase verbosity. true, false are supported values",
		"DUFFLE_ACI_DRIVER_CLIENT_ID":             "AAD Client ID for Azure account authentication - used to authenticate to Azure for ACI creation",
		"DUFFLE_ACI_DRIVER_CLIENT_SECRET":         "AAD Client Secret for Azure account authentication - used to authenticate to Azure for ACI creation",
		"DUFFLE_ACI_DRIVER_TENANT_ID":             "Azure AAD Tenant Id Azure account authentication - used to authenticate to Azure for ACI creation",
		"DUFFLE_ACI_DRIVER_SUBSCRIPTION_ID":       "Azure Subscription Id - this is the subscription to be used for ACI creation, if not specified the default subscription is used",
		"DUFFLE_ACI_DRIVER_APP_ID":                "Azure Application Id - this is the application to be used to authenticate to Azure",
		"DUFFLE_ACI_DRIVER_RESOURCE_GROUP":        "The name of the existing Resource Group to create the ACI instance in, if not specified a Resource Group will be created",
		"DUFFLE_ACI_DRIVER_LOCATION":              "The location to create the ACI Instance in",
		"DUFFLE_ACI_DRIVER_NAME":                  "The name of the ACI instance to create - if not specified a name will be generated",
		"DUFFLE_ACI_DRIVER_DELETE_RESOURCES":      "Delete RG and ACI instance created - default is true useful to set to false for debugging - only deletes RG if it was created by the driver",
		"DUFFLE_ACI_DRIVER_MSI_TYPE":              "This can be set to user or system",
		"DUFFLE_ACI_DRIVER_SYSTEM_MSI_ROLE":       "The role to be asssigned to System MSI User - used if DUFFLE_ACI_DRIVER_ACI_MSI_TYPE == system, if this is null or empty then the role defaults to contributor",
		"DUFFLE_ACI_DRIVER_SYSTEM_MSI_SCOPE":      "The scope to apply the role to System MSI User - will attempt to set scope to the  Resource Group that the ACI Instance is being created in if not set",
		"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID":  "The resource Id of the MSI User - required if DUFFLE_ACI_DRIVER_ACI_MSI_TYPE == User ",
		"DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS": "If this is set to true the credentials used to Launch the Driver are propagated to the invocation image in an ENV variable DUFFLE_ACI_DRIVER prefix will be relaced with AZURE_, default is false",
	}
}

// NewACIDriver creates a new ACI Driver instance
func NewACIDriver() (driver.Driver, error) {
	d := &aciDriver{
		config:      map[string]string{},
		msiResource: azure.Resource{},
	}
	d.inCloudShell = len(os.Getenv("ACC_CLOUD")) > 0
	d.log("In Cloud Shell:", d.inCloudShell)
	// if d.inCloudShell {
	// 	d.getSettingsFromCloudShellConfig()
	// }

	for env := range d.Config() {
		d.config[env] = os.Getenv(env)
	}
	d.verbose = len(d.config["DUFFLE_ACI_DRIVER_VERBOSE"]) > 0 && strings.ToLower(d.config["DUFFLE_ACI_DRIVER_VERBOSE"]) == "true"
	fmt.Println("VERBOSE:", d.config["DUFFLE_ACI_DRIVER_VERBOSE"])
	d.log("verbose:", d.verbose)
	d.deleteACIResources = true
	if len(d.config["DUFFLE_ACI_DRIVER_DELETE_RESOURCES"]) > 0 && (strings.ToLower(d.config["DUFFLE_ACI_DRIVER_DELETE_RESOURCES"]) == "false") {
		d.deleteACIResources = false
	}

	d.log("Delete Resources:", d.deleteACIResources)
	d.clientID = d.config["DUFFLE_ACI_DRIVER_CLIENT_ID"]
	d.log("clientID:", d.clientID)
	d.clientSecret = d.config["DUFFLE_ACI_DRIVER_CLIENT_SECRET"]
	d.log("clientSecret:", d.clientSecret)
	d.tenantID = d.config["DUFFLE_ACI_DRIVER_TENANT_ID"]
	d.log("tenantID:", d.tenantID)
	d.applicationID = d.config["DUFFLE_ACI_DRIVER_APP_ID"]
	d.log("applicationID:", d.applicationID)
	d.subscriptionID = d.config["DUFFLE_ACI_DRIVER_SUBSCRIPTION_ID"]
	d.log("Subscription:", d.subscriptionID)
	// TODO get default subscription from az cli
	d.aciRG = d.config["DUFFLE_ACI_DRIVER_RESOURCE_GROUP"]
	d.log("Resource Group:", d.aciRG)
	d.aciLocation = strings.ToLower(strings.Replace(d.config["DUFFLE_ACI_DRIVER_LOCATION"], " ", "", -1))
	d.log("Location:", d.aciLocation)
	if len(d.aciRG) == 0 && len(d.aciLocation) == 0 {
		// TODO check if running in cloudshell or cli configured and defaults are set
		return nil, errors.New("ACI Driver requires DUFFLE_ACI_DRIVER_LOCATION environment variable or an existing Resource Group in DUFFLE_ACI_DRIVER_RESOURCE_GROUP")
	}

	d.createRG = len(d.aciRG) == 0
	if d.createRG {
		d.aciRG = uuid.New().String()
		d.log("Resource Group:", d.aciRG)
	}

	d.aciName = d.config["DUFFLE_ACI_DRIVER_NAME"]
	if len(d.aciName) == 0 {
		d.aciName = fmt.Sprintf("duffle-%s", uuid.New().String())
	}

	d.log("ACI Name:", d.aciName)
	if len(d.config["DUFFLE_ACI_DRIVER_MSI_TYPE"]) > 0 {
		switch strings.ToLower(d.config["DUFFLE_ACI_DRIVER_MSI_TYPE"]) {
		case "system":
			d.msiType = "system"

		case "user":
			d.msiType = "user"
			d.userMSIResourceID = d.config["DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID"]
			if len(d.userMSIResourceID) == 0 {
				return nil, errors.New("ACI Driver requires DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable when DUFFLE_ACI_DRIVER_MSI_TYPE is set to user")
			}

			resource, err := azure.ParseResourceID(d.userMSIResourceID)
			if err != nil {
				return nil, fmt.Errorf("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable parsing error: %v", err)
			}

			if strings.ToLower(resource.Provider) != "microsoft.managedidentity" || strings.ToLower(resource.ResourceType) != "userassignedidentities" {
				return nil, fmt.Errorf("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: %s/%s", resource.Provider, resource.ResourceType)
			}

			d.msiResource = resource
			d.log("User MSI Resource ID:", d.userMSIResourceID)

		default:
			return nil, fmt.Errorf("DUFFLE_ACI_DRIVER_MSI_TYPE environment variable unknown value: %s", d.config["DUFFLE_ACI_DRIVER_MSI_TYPE"])
		}
		d.log("MSI Type:", d.msiType)
	}

	d.propagateCredentials = len(d.config["DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS"]) > 0 && strings.ToLower(d.config["DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS"]) == "true"
	d.log("Propagate Credentials:", d.propagateCredentials)
	return d, nil
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

	err := d.setAuthorizer()
	if err != nil {
		return fmt.Errorf("cannot Login To Azure: %v", err)
	}

	err = d.setAzureSubscriptionID()
	if err != nil {
		return fmt.Errorf("cannot set Azure subscription: %v", err)
	}

	err = d.createACIInstance(op)
	if err != nil {
		return fmt.Errorf("creating ACI instance failed: %v", err)
	}

	return nil
}

func (d *aciDriver) setAuthorizer() error {

	// Attempt to login with Service Principal
	if len(d.clientID) != 0 && len(d.clientSecret) != 0 && len(d.tenantID) != 0 {
		d.log("Attempting to Login with Service Principal")
		clientCredentailsConfig := auth.NewClientCredentialsConfig(d.clientID, d.clientSecret, d.tenantID)
		authorizer, err := clientCredentailsConfig.Authorizer()
		if err != nil {
			return fmt.Errorf("Attempt to set Authorizer with Service Principal failed: %v", err)
		}

		d.authorizer = authorizer
		return nil
	}

	// Attempt to login with Device Code
	if len(d.applicationID) != 0 && len(d.tenantID) != 0 {
		d.log("Attempting to Login with Device Code")
		deviceFlowConfig := auth.NewDeviceFlowConfig(d.applicationID, d.tenantID)
		var err error
		d.oauthTokenProvider, err = deviceFlowConfig.ServicePrincipalToken()
		if err != nil {
			return fmt.Errorf("failed to get oauth token from device flow: %v", err)
		}

		fmt.Println("Logged in with Device Code")
		d.authorizer = autorest.NewBearerAuthorizer(d.oauthTokenProvider)
		d.clientLogin = "devicecode"
		return nil
	}

	// Attempt to use token from CloudShell
	if d.inCloudShell {
		d.log("Attempting to Login with CloudShell")
		token, err := d.getCloudShellToken()
		if err != nil {
			return fmt.Errorf("Attempt to get CloudShell token failed: %v", err)
		}

		d.oauthTokenProvider = token
		d.authorizer = autorest.NewBearerAuthorizer(token)
		d.clientLogin = "cloudshell"
		return nil
	}

	// Attempt to login with MSI
	if checkForMSIEndpoint() {
		d.log("Attempting to Login with MSI")
		msiConfig := auth.NewMSIConfig()
		authorizer, err := msiConfig.Authorizer()
		if err != nil {
			return fmt.Errorf("Attempt to set Authorizer with MSI failed: %v", err)
		}

		d.authorizer = authorizer
		return nil
	}

	// Attempt to Login using azure CLI
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		d.authorizer = authorizer
		d.clientLogin = "cli"
		return nil
	}

	return fmt.Errorf("Cannot login to Azure - no valid credentials provided or available, failed to login with Azure cli: %v", err)
}

func (d *aciDriver) setAzureSubscriptionID() error {

	subscriptionsClient := d.getSubscriptionsClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if len(d.subscriptionID) != 0 {
		d.log("Checking for Subscription ID:", d.subscriptionID)
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
			d.log("Setting Subscription to", subscriptionID)
			d.subscriptionID = subscriptionID
		} else {
			return errors.New("Cannot find a subscription")
		}

	}

	return nil
}

func (d *aciDriver) createACIInstance(op *driver.Operation) error {
	// GET ACI Config

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	groupsClient := d.getGroupsClient()
	if !d.createRG {
		rg, err := groupsClient.Get(ctx, d.aciRG)
		if err != nil {
			return fmt.Errorf("Checking for existing resource group %s failed with error: %v", d.aciRG, err)
		}

		if len(d.aciLocation) == 0 {
			d.log("Setting aci Location to RG Location:", *rg.Location)
			d.aciLocation = *rg.Location
		}

	}

	// Check that location supports ACI

	providersClient := d.getProvidersClient()
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
		d.log("Creating Resource Group")
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
				d.log("Deleting Resource Group")
				future, err := groupsClient.Delete(ctx, d.aciRG)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to execute delete resource group %s error: %v\n", d.aciRG, err)
				}

				err = future.WaitForCompletionRef(ctx, groupsClient.Client)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to delete resource group %s error: %v\n", d.aciRG, err)
				} else {
					d.log("Deleted Resource Group ", d.aciRG)
				}

			}

		}()
	}

	// TODO ACI does not support file copy
	// Does not support files because ACI does not support file copy yet
	if len(op.Files) > 0 {
		for k, v := range op.Files {
			d.log("File", k, "Value", v)
		}
		// Ignore Image Map if its empty
		if v, e := op.Files["/cnab/app/image-map.json"]; e && (len(v) > 0 && v != "{}") || !e || len(op.Files) > 1 {
			return errors.New("ACI Driver does not support files")
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

	}

	for k, v := range op.Environment {

		// Need to check if any of the env variables already exist in case any propagated credentials are being overridden
		for _, ev := range env {
			if k == *ev.Name {
				ev.SecureValue = to.StringPtr(strings.Replace(v, "'", "''", -1))
				d.log("Updating Container Group Environment Variable: Name:", k, "to Value:", v)
				continue
			}

		}
		env = append(env, containerinstance.EnvironmentVariable{
			Name:        to.StringPtr(k),
			SecureValue: to.StringPtr(strings.Replace(v, "'", "''", -1)),
		})
		d.log("Setting Container Group Environment Variable: Name:", k, "Value:", v)
	}
	identity, err := d.getContainerIdentity(ctx, d.aciRG)
	if err != nil {
		return fmt.Errorf("Failed to get container Identity:%v", err)
	}

	_, err = d.createInstance(d.aciName, d.aciLocation, d.aciRG, op.Image, env, *identity)
	if err != nil {
		return fmt.Errorf("Error creating ACI Instance:%v", err)
	}

	// TODO: Check if ACR under ACI supports MSI
	// TODO: Login to ACR if the registry is azurecr.io
	// TODO: Add support for private registry

	if d.deleteACIResources {
		defer func() {
			d.log("Deleting Container Instance")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			containerGroupsClient := d.getContainerGroupsClient()
			_, err := containerGroupsClient.Delete(ctx, d.aciRG, d.aciName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to delete container error: %v\n", err)
			}

			d.log("Deleted Container ", d.aciName)
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
		d.log("Getting Container State")
		state, err := d.getContainerState(d.aciRG, d.aciName)
		if err != nil {
			return fmt.Errorf("Error getting container state :%v", err)
		}

		if strings.Compare(state, "Running") == 0 {
			linesOutput, err = d.getContainerLogs(ctx, d.aciRG, d.aciName, linesOutput)
			if err != nil {
				return fmt.Errorf("Error getting container logs :%v", err)
			}

			d.log("Sleeping")
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

	d.log("Container terminated successfully")
	return nil
}

// This will only work if the logs dont get truncated because of size.
func (d *aciDriver) getContainerLogs(ctx context.Context, aciRG string, aciName string, linesOutput int) (int, error) {
	d.log("Getting Logs")
	containerClient := d.getContainerClient()
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

		d.setSystemMSIRoleAndScope()

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
		userAssignedIdentitiesClient := d.getUserAssignedIdentitiesClient(d.msiResource.SubscriptionID)
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

func (d *aciDriver) getSubscriptionsClient() subscriptions.Client {
	subscriptionClient := subscriptions.NewClient()
	subscriptionClient.Authorizer = d.authorizer
	subscriptionClient.AddToUserAgent(userAgent)
	return subscriptionClient
}

func (d *aciDriver) getRoleDefinitionsClient(subscriptionID string) authorization.RoleDefinitionsClient {
	roleDefinitionsClient := authorization.NewRoleDefinitionsClient(subscriptionID)
	roleDefinitionsClient.Authorizer = d.authorizer
	roleDefinitionsClient.AddToUserAgent(userAgent)
	return roleDefinitionsClient
}

func (d *aciDriver) getRoleAssignmentClient(subscriptionID string) authorization.RoleAssignmentsClient {
	roleAssignmentsClient := authorization.NewRoleAssignmentsClient(subscriptionID)
	roleAssignmentsClient.Authorizer = d.authorizer
	roleAssignmentsClient.AddToUserAgent(userAgent)
	return roleAssignmentsClient
}

func (d *aciDriver) getUserAssignedIdentitiesClient(subscriptionID string) msi.UserAssignedIdentitiesClient {
	userAssignedIdentitiesClient := msi.NewUserAssignedIdentitiesClient(subscriptionID)
	userAssignedIdentitiesClient.Authorizer = d.authorizer
	userAssignedIdentitiesClient.AddToUserAgent(userAgent)
	return userAssignedIdentitiesClient
}

func (d *aciDriver) getContainerGroupsClient() containerinstance.ContainerGroupsClient {
	containerGroupsClient := containerinstance.NewContainerGroupsClient(d.subscriptionID)
	containerGroupsClient.Authorizer = d.authorizer
	containerGroupsClient.AddToUserAgent(userAgent)
	return containerGroupsClient
}

func (d *aciDriver) getContainerClient() containerinstance.ContainerClient {
	containerClient := containerinstance.NewContainerClient(d.subscriptionID)
	containerClient.Authorizer = d.authorizer
	containerClient.AddToUserAgent(userAgent)
	return containerClient
}

func (d *aciDriver) getGroupsClient() resources.GroupsClient {
	groupsClient := resources.NewGroupsClient(d.subscriptionID)
	groupsClient.Authorizer = d.authorizer
	groupsClient.AddToUserAgent(userAgent)
	return groupsClient
}

func (d *aciDriver) getProvidersClient() resources.ProvidersClient {
	providersClient := resources.NewProvidersClient(d.subscriptionID)
	providersClient.Authorizer = d.authorizer
	providersClient.AddToUserAgent(userAgent)
	return providersClient
}

func (d *aciDriver) getContainerState(aciRG string, aciName string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	containerGroupsClient := d.getContainerGroupsClient()
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

func checkForMSIEndpoint() bool {
	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	_, err := client.Head(msiTokenEndpoint)
	return err == nil
}

func (d *aciDriver) createContainerGroup(aciName string, aciRG string, containerGroup containerinstance.ContainerGroup) (containerinstance.ContainerGroup, error) {
	containerGroupsClient := d.getContainerGroupsClient()
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

func (d *aciDriver) createInstance(aciName string, aciLocation string, aciRG string, image string, env []containerinstance.EnvironmentVariable, identity identityDetails) (*containerinstance.ContainerGroup, error) {
	// ARM does not yet support the ability to create a System MSI and assign role and scope on creation
	// so if the MSI type is system assigned then need to create the ACI Instance first with an alpine instance in order to create the identity and then assign permissions
	// The created ACI is then updated to execute the Invocation Image
	if identity.MSIType == "system" {
		d.log("Creating ACI to create System Identity")
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

	d.log("Creating ACI for CNAB action")
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
						},
					},
				},
			},
		})
	if err != nil {
		return nil, fmt.Errorf("Error Creating Container Group: %v", err)
	}

	return &containerGroup, nil
}

func (d *aciDriver) setUpSystemMSIRBAC(principalID *string, scope string, role string) error {
	d.log("Setting up System MSI Scope ", scope, "Role ", role)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	roleDefinitionsClient := d.getRoleDefinitionsClient(d.subscriptionID)
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

	d.log("RoleDefinitionId", roleDefinitionID)
	// Wait for principal to be available
	attempts := 5
	var err error
	for i := 0; i < attempts; i++ {
		d.log("Creating RoleAssignment Attempt", i)
		roleAssignmentsClient := d.getRoleAssignmentClient(d.subscriptionID)
		_, raerror := roleAssignmentsClient.Create(ctx, scope, uuid.New().String(), authorization.RoleAssignmentCreateParameters{
			Properties: &authorization.RoleAssignmentProperties{
				RoleDefinitionID: &roleDefinitionID,
				PrincipalID:      principalID,
			},
		})
		if raerror != nil {
			err = fmt.Errorf("Error creating RoleAssignment Role:%s for Scope:%s Error: %v", role, scope, raerror)
			d.log("Creating RoleAssignment Attempt:", i, "Error:", err)
			time.Sleep(20 * time.Second)
			continue
		}

		err = raerror
		break
	}

	return err
}

func (d *aciDriver) log(message ...interface{}) {
	if d.verbose {
		fmt.Println(message...)
	}

}

type identityDetails struct {
	MSIType  string
	Identity *containerinstance.ContainerGroupIdentity
	Scope    *string
	Role     *string
}

func (d *aciDriver) getCloudShellToken() (*adal.Token, error) {

	MSIEndpoint := os.Getenv("MSI_ENDPOINT")
	d.log("CloudShell MSI Endpoint", MSIEndpoint)
	if len(MSIEndpoint) == 0 {
		return nil, errors.New("MSI_ENDPOINT environment variable not set")
	}

	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	req, err := http.NewRequest("GET", "http://localhost:50342/oauth2/token", nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating HTTP Request to CloudShell Token: %v", err)
	}

	req.Header.Set("Metadata", "true")
	query := req.URL.Query()
	query.Add("api-version", "2018-02-01")
	query.Add("resource", "https://management.azure.com/")
	req.URL.RawQuery = query.Encode()
	d.log("Token Query", query.Encode())
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error getting CloudShell Token: %v", err)
	}

	defer resp.Body.Close()
	rawResp, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if err != nil {
			return nil, fmt.Errorf("Error getting CloudShell Token. Status Code:'%d'. Failed reading response body error: %v", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("Error getting Token. Status Code:'%d'. Response body: %s", resp.StatusCode, string(rawResp))
	}

	var token adal.Token
	err = json.Unmarshal(rawResp, &token)
	if err != nil {
		return nil, fmt.Errorf("Error deserialising CloudShell Token Status Code: '%d'. Token: %s", resp.StatusCode, string(rawResp))
	}

	return &token, nil
}

func (d *aciDriver) createMSIEnvVars(env []containerinstance.EnvironmentVariable) []containerinstance.EnvironmentVariable {

	if len(d.msiType) > 0 {
		name := "AZURE_MSI_TYPE"
		env = append(env, containerinstance.EnvironmentVariable{
			Name:        &name,
			SecureValue: &d.msiType,
		})
		d.log("Setting Container Group Environment Variable: Name:", name, "Value:", d.msiType)
		if d.msiType == "user" {
			name := "AZURE_USER_MSI_RESOURCE_ID"
			env = append(env, containerinstance.EnvironmentVariable{
				Name:        &name,
				SecureValue: &d.userMSIResourceID,
			})
			d.log("Setting Container Group Environment Variable: Name:", name, "Value:", d.userMSIResourceID)
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
			d.log("Setting Container Group Environment Variable: Name:", v, "Value:", value)
		}
	}
	return env
}

func (d *aciDriver) createCredentialEnvVars(env []containerinstance.EnvironmentVariable) ([]containerinstance.EnvironmentVariable, error) {

	if len(d.clientLogin) > 0 {
		name := "AZURE_OAUTH_TOKEN"
		var token string
		if d.clientLogin == "cloudshell" || d.clientLogin == "devicecode" {
			d.log(fmt.Sprintf("Propagating OAuth Token from %s login", d.clientLogin))
			token = d.oauthTokenProvider.OAuthToken()
		}

		if d.clientLogin == "cli" {
			d.log("Propagating OAuth Token from cli")
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
		d.log("Setting Container Group Environment Variable: Name:", name, "Value:", token)
	}

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
			d.log("Setting Container Group Environment Variable: Name:", v, "Value:", value)
		}
	}
	return env, nil
}

func (d *aciDriver) getFieldValue(field string) string {
	r := reflect.ValueOf(d)
	return reflect.Indirect(r).FieldByName(field).String()
}

func (d *aciDriver) setSystemMSIRoleAndScope() {
	scope := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s", d.subscriptionID, d.aciRG)
	role := "Contributor"
	if len(d.config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_ROLE"]) > 0 {
		role = d.config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_ROLE"]
	}

	d.log("MSI Role:", role)
	d.systemMSIScope = scope
	if len(d.config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_SCOPE"]) > 0 {
		scope = d.config["DUFFLE_ACI_DRIVER_SYSTEM_MSI_SCOPE"]
	}

	d.systemMSIRole = role
	d.log("MSI Scope:", scope)
}
