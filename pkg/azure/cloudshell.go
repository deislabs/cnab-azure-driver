package azure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-ini/ini.v1"
)

const (
	azureCLIConfigFileName = "config"
)

type checkAccessResponse []accessResponse

type accessResponse struct {
	AccessDecision string `json:"accessDecision"`
}
type checkAccessRequest struct {
	Subject subject  `json:"Subject"`
	Actions []action `json:"Actions"`
}
type action struct {
	ID string `json:"Id"`
}
type subject struct {
	Attributes attributes `json:"Attributes"`
}
type attributes struct {
	ObjectID string `json:"ObjectId"`
}

type cloudrive struct {
	StorageAccountResourceID string `json:"storageAccountResourceId"`
	FileShareName            string `json:"fileShareName"`
	Size                     int    `json:"diskSizeInGB"`
}

type adtoken struct {
	Oid string `json:"oid"`
	Aud string `json:"aud"`
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

	// Do not throw errors as just return an empty struct
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
	// Log errors and return empty string if cannot read from the az config
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
	client, err := GetStorageAccountsClient(resource.SubscriptionID, authorizer, userAgent)
	if err != nil {
		return nil, fmt.Errorf("Error getting Storage Accounts Client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := client.ListKeys(ctx, resource.ResourceGroup, resource.ResourceName, "")
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

	// return empty string if there are any errors
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

// CheckCanAccessResource checks to see if the user can create a specific
func CheckCanAccessResource(actionID string, scope string) (bool, error) {
	if !IsInCloudShell() {
		return false, errors.New("Not Running in CloudShell")
	}
	adalToken, err := GetCloudShellToken()
	if err != nil {
		return false, fmt.Errorf("Error Getting CloudShellToken: %v", err)
	}
	oid, err := getFromToken(adalToken.AccessToken, "oid")
	if err != nil {
		return false, fmt.Errorf("failed to get Oid: %v ", err)
	}
	accessCheck := checkAccessRequest{
		Actions: []action{
			{
				ID: actionID,
			},
		},
		Subject: subject{
			Attributes: attributes{
				ObjectID: oid,
			},
		},
	}
	payload, err := json.Marshal(accessCheck)
	if err != nil {
		return false, fmt.Errorf("failed to serialise checkaccess payload: %v ", err)
	}
	log.Debug("Check Access POST Body ", string(payload))
	return makeCheckAccessRequest(payload, scope)
}
func getFromToken(accessToken string, parameter string) (string, error) {
	bearerToken := strings.Split(accessToken, ".")[1]
	if len(bearerToken) == 0 {
		return "", errors.New("Failed to get bearer token from CloudShell Token")
	}
	token, err := base64.RawStdEncoding.DecodeString(bearerToken)
	if err != nil {
		return "", fmt.Errorf("Failed to decode Bearer Token: %v ", err)
	}
	var adToken map[string]interface{}
	if err := json.Unmarshal(token, &adToken); err != nil {
		return "", fmt.Errorf("Failed to unmarshall CloudShell token: %v ", err)
	}
	parameterValue, hasParameter := adToken[parameter]
	if hasParameter == false {
		return "", errors.New("Requested token parameter not present")
	}
	return parameterValue.(string), err
}
func makeCheckAccessRequest(payload []byte, scope string) (bool, error) {

	var err error
	var response []byte
	adalToken, err := GetCloudShellToken()
	if err != nil {
		return false, fmt.Errorf("Error Getting CloudShellToken: %v", err)
	}
	audUrl, err := getFromToken(adalToken.AccessToken, "aud")
	if err != nil {
		audUrl = "https://management.azure.com/"
	}
retry:
	for i := 1; i < 4; i++ {
		timeout := time.Duration(time.Duration(i) * time.Second)
		client := http.Client{
			Timeout: timeout,
		}
		url := fmt.Sprintf("%s%s/providers/Microsoft.Authorization/CheckAccess", audUrl, scope)
		log.Debug("Check Access URL: ", url)
		var req *http.Request
		req, err = http.NewRequest("POST", url, bytes.NewBuffer(payload))
		if err != nil {
			break retry
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", adalToken.AccessToken))
		req.Header.Set("content-type", "application/json")
		query := req.URL.Query()
		query.Add("api-version", "2018-09-01-preview")
		req.URL.RawQuery = query.Encode()
		var resp *http.Response
		resp, err = client.Do(req)
		if err != nil {
			break retry
		}

		var rawResp []byte
		rawResp, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			break retry
		}

		defer resp.Body.Close()
		response = rawResp
		log.Debug("Check Access HTTP Status Code: ", resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusOK:
			break retry
		case http.StatusForbidden:
			// User does not have permission to call CheckAccess so just return true, the operation may still succeed
			return true, nil
		default:
			log.Debug(fmt.Sprintf("Error checking access HTTP Status: '%d'. Response Body: %s", resp.StatusCode, string(rawResp)))
			err = fmt.Errorf("Error checking access HTTP Status: '%d'. Response Body: %s", resp.StatusCode, string(rawResp))
		}
	}

	if err != nil {
		return false, fmt.Errorf("Error Checking  Access: %v", err)
	}

	log.Debug(fmt.Sprintf("Check Access Response Body: %s", string(response)))
	var checkaccessresponse checkAccessResponse
	err = json.Unmarshal(response, &checkaccessresponse)
	if err != nil {
		return false, fmt.Errorf("Error Unmarshalling Access Response: %v", err)
	}

	return strings.ToLower(checkaccessresponse[0].AccessDecision) == "allowed", nil
}
