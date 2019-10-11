package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-ini/ini.v1"
	"os"
	"path"
	"strings"
)

const (
	azureCLIConfigFileName = "config"
)

type cloudrive struct {
	StorageAccountResourceID string `json:"storageAccountResourceId"`
	FileShareName            string `json:"fileShareName"`
	Size                     int    `json:"diskSizeInGB"`
}

// FileShareDetails contains details of the clouddrive FileShare
type FileShareDetails struct {
	Name               string
	StorageAccountName string
	StorageAccountKey  string
}

// IsInCloudShell checks if we are currently running in CloudShell
func IsInCloudShell() bool {
	return len(os.Getenv("ACC_CLOUD")) > 0
}

// GetCloudShellLocation returns the location that CloudShell is running in
func GetCloudShellLocation() string {
	if !IsInCloudShell() {
		return ""
	}
	return os.Getenv("ACC_LOCATION")
}

func getSubscriptionsfromCLIProfile() *[]cli.Subscription {

	subscriptions := []cli.Subscription{}

	if !IsInCloudShell() {
		return &subscriptions
	}

	profilePath, err := cli.ProfilePath()
	if err != nil {
		log.Debug("Failed to get cli ProfilePath: ", err)
		return &subscriptions
	}

	profile, err := cli.LoadProfile(profilePath)
	if err != nil {
		log.Debug("Failed to get cli Profile: ", err)
		return &subscriptions

	}
	return &profile.Subscriptions
}

// GetTenantIDFromCliProfile gets tenant id from az cli profile
func GetTenantIDFromCliProfile() string {
	if !IsInCloudShell() {
		return ""
	}

	subscriptions := getSubscriptionsfromCLIProfile()
	for _, subscription := range *subscriptions {
		if strings.ToLower(subscription.EnvironmentName) == "azurecloud" && subscription.IsDefault {
			return subscription.TenantID
		}
	}
	return ""
}

// GetSubscriptionIDFromCliProfile gets default subscription id from az cli profile
func GetSubscriptionIDFromCliProfile() string {
	if !IsInCloudShell() {
		return ""
	}

	subscriptions := getSubscriptionsfromCLIProfile()
	for _, subscription := range *subscriptions {
		if strings.ToLower(subscription.EnvironmentName) == "azurecloud" && subscription.IsDefault {
			log.Debug("Subscription from cli profile is: ", subscription.ID)
			return subscription.ID
		}
	}
	return ""
}

// TryGetRGandLocation tries to get Resource Group and Location Information from az defaults and ACC_env var
func TryGetRGandLocation() (rg string, location string) {
	if !IsInCloudShell() {
		return
	}
	// Get values from ENV , if not set try the config ini
	rg, location = getRGAndLocationFromEnv()
	if len(rg) == 0 || len(location) == 0 {
		crg, clocation := getRGAndLocationFromConfig()
		if len(rg) == 0 {
			rg = crg
		}
		if len(location) == 0 {
			location = clocation
		}
	}

	// If neither value is set get the location of cloudshell
	if len(location) == 0 && len(rg) == 0 {
		location = os.Getenv("ACC_LOCATION")
	}
	log.Debug("Resource Group from CloudShell: ", rg)
	location = strings.ToLower(strings.Replace(location, " ", "", -1))
	log.Debug("Location from CloudShell: ", location)
	return
}
func getRGAndLocationFromEnv() (rg string, location string) {
	rg = os.Getenv("AZURE_DEFAULTS_GROUP")
	location = os.Getenv("AZURE_DEFAULTS_LOCATION")
	return
}
func getRGAndLocationFromConfig() (rg string, location string) {
	path, err := getCLIConfig()
	if err != nil {
		log.Debug("failed to get cli config path:", err)
		return
	}

	if _, err := os.Stat(path); err != nil {
		log.Debug("failed to get cli config:", err)
		return
	}

	cfg, err := ini.Load(path)
	if err != nil {
		log.Debug("failed to parse config ", err)
		return
	}
	rg = cfg.Section("defaults").Key("group").String()
	location = cfg.Section("defaults").Key("location").String()
	return
}

// GetCloudDriveDetails gets the details of the clouddrive cloudshare
func GetCloudDriveDetails(userAgent string) (*FileShareDetails, error) {
	if !IsInCloudShell() {
		return nil, errors.New("Not Running in CloudShell")
	}

	clouddriveConfig, err := getCloudDriveConfig()
	if err != nil {
		return nil, err
	}

	resource := parseResourceID(clouddriveConfig.StorageAccountResourceID)
	if resource == nil {
		return nil, errors.New("Failed to Parse resource Id")
	}

	token, err := GetCloudShellToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get CloudShell Token: %s", err)
	}

	authorizer := autorest.NewBearerAuthorizer(token)
	client := GetStorageAccountsClient(resource.SubscriptionID, authorizer, userAgent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := client.ListKeys(ctx, resource.ResourceGroup, resource.ResourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get strage account keys: %s", err)
	}

	return &FileShareDetails{
		Name:               clouddriveConfig.FileShareName,
		StorageAccountName: resource.ResourceName,
		StorageAccountKey:  *(((*result.Keys)[0]).Value),
	}, nil
}

// GetCloudDriveResourceGroup gets the resource group name associated with the clouddrive used in CloudShell
func GetCloudDriveResourceGroup() string {
	if !IsInCloudShell() {
		return ""
	}

	clouddriveConfig, err := getCloudDriveConfig()
	if err != nil {
		log.Debug("Error getting clouddrive config: ", err)
		return ""
	}

	resource, err := azure.ParseResourceID(clouddriveConfig.StorageAccountResourceID)
	if err != nil {
		return ""
	}

	return resource.ResourceGroup

}

func parseResourceID(resourceID string) *azure.Resource {
	resource, err := azure.ParseResourceID(resourceID)
	if err != nil {
		log.Debug("failed to parse StorageAccountResourceID: ", err)
	}

	return &resource
}

func getCloudDriveConfig() (*cloudrive, error) {
	clouddriveConfig := cloudrive{}
	if err := json.Unmarshal([]byte(os.Getenv("ACC_STORAGE_PROFILE")), &clouddriveConfig); err != nil {
		return &clouddriveConfig, fmt.Errorf("failed to unmarshall ACC_STORAGE_PROFILE: %v ", err)
	}

	return &clouddriveConfig, nil
}

func getCLIConfig() (string, error) {
	if cfgDir := os.Getenv("AZURE_CONFIG_DIR"); cfgDir != "" {
		return path.Join(cfgDir, azureCLIConfigFileName), nil
	}
	return homedir.Expand("~/.azure/" + azureCLIConfigFileName)
}
