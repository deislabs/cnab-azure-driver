package azure

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	log "github.com/sirupsen/logrus"
)

const (
	msiTokenEndpoint = "http://169.254.169.254/metadata/identity/oauth2/token"
)

// LoginType is the type of login used
type LoginType int

const (
	// ServicePrincipal - logged in using Service Principal
	ServicePrincipal LoginType = iota
	// DeviceCode - logged in using DeviceCode Flow
	DeviceCode
	// CloudShell - logged in using DeviceCode Flow
	CloudShell
	// MSI - logged in using DeviceCode Flow
	MSI
	// CLI - logged in using DeviceCode Flow
	CLI
)

// LoginInfo contains Azure login information
type LoginInfo struct {
	Authorizer         autorest.Authorizer
	LoginType          LoginType
	OAuthTokenProvider adal.OAuthTokenProvider
}

// LoginToAzure attempts to login to azure
func LoginToAzure(clientID string, clientSecret string, tenantID string, applicationID string) (LoginInfo, error) {

	var loginInfo LoginInfo
	var err error
	// Attempt to login with Service Principal
	if len(clientID) != 0 && len(clientSecret) != 0 && len(tenantID) != 0 {
		log.Debug("Attempting to Login with Service Principal")
		clientCredentailsConfig := auth.NewClientCredentialsConfig(clientID, clientSecret, tenantID)
		loginInfo.Authorizer, err = clientCredentailsConfig.Authorizer()
		if err != nil {
			return loginInfo, fmt.Errorf("Attempt to set Authorizer with Service Principal failed: %v", err)
		}

		log.Debug("Logged in with Service Principal")
		loginInfo.LoginType = ServicePrincipal
		return loginInfo, nil
	}

	// Attempt to login with Device Code
	if len(applicationID) != 0 && len(tenantID) != 0 {
		log.Debug("Attempting to Login with Device Code")
		deviceFlowConfig := auth.NewDeviceFlowConfig(applicationID, tenantID)
		loginInfo.OAuthTokenProvider, err = deviceFlowConfig.ServicePrincipalToken()
		if err != nil {
			return loginInfo, fmt.Errorf("failed to get oauth token from device flow: %v", err)
		}

		log.Debug("Logged in with Device Code")
		loginInfo.Authorizer = autorest.NewBearerAuthorizer(loginInfo.OAuthTokenProvider)
		loginInfo.LoginType = DeviceCode
		return loginInfo, nil
	}

	// Attempt to use token from CloudShell
	if IsInCloudShell() {
		log.Debug("Attempting to Login with CloudShell")
		loginInfo.OAuthTokenProvider, err = GetCloudShellToken()
		if err != nil {
			return loginInfo, fmt.Errorf("Attempt to get CloudShell token failed: %v", err)
		}

		log.Debug("Logged in with CloudShell")
		loginInfo.Authorizer = autorest.NewBearerAuthorizer(loginInfo.OAuthTokenProvider)
		loginInfo.LoginType = CloudShell
		return loginInfo, nil
	}

	// Attempt to login with MSI
	if checkForMSIEndpoint() {
		log.Debug("Attempting to Login with MSI")
		msiConfig := auth.NewMSIConfig()
		loginInfo.Authorizer, err = msiConfig.Authorizer()
		if err != nil {
			return loginInfo, fmt.Errorf("Attempt to set Authorizer with MSI failed: %v", err)
		}
		loginInfo.LoginType = MSI
		log.Debug("Logged in with MSI")
		return loginInfo, nil
	}

	// Attempt to Login using azure CLI
	log.Debug("Attempting to Login with az cli")
	loginInfo.Authorizer, err = auth.NewAuthorizerFromCLI()
	if err == nil {
		loginInfo.LoginType = CLI
		log.Debug("Logged in with CLI")
		return loginInfo, nil
	}

	return loginInfo, fmt.Errorf("Cannot login to Azure - no valid credentials provided or available, failed to login with Azure cli: %v", err)
}

//GetCloudShellToken gets the CloudShell Token
func GetCloudShellToken() (*adal.Token, error) {

	MSIEndpoint := os.Getenv("MSI_ENDPOINT")
	log.Debug("CloudShell MSI Endpoint: ", MSIEndpoint)
	if len(MSIEndpoint) == 0 {
		return nil, errors.New("MSI_ENDPOINT environment variable not set")
	}

	MSIAudience := os.Getenv("CNAB_AZURE_MSI_AUDIENCE")
	if len(MSIAudience) == 0 {
		MSIAudience = "https://management.azure.com/"
	}
	log.Debug("CloudShell MSI Audience: ", MSIAudience)

	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	req, err := http.NewRequest("GET", MSIEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating HTTP Request to CloudShell Token: %v", err)
	}

	req.Header.Set("Metadata", "true")
	query := req.URL.Query()
	query.Add("api-version", "2018-02-01")
	query.Add("resource", MSIAudience)
	req.URL.RawQuery = query.Encode()
	log.Debug("Cloud Shell Token URI: ", req.RequestURI)
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
func checkForMSIEndpoint() bool {
	var err error
	for i := 1; i < 4; i++ {
		timeout := time.Duration(time.Duration(i) * time.Second)
		client := http.Client{
			Timeout: timeout,
		}
		_, err = client.Head(msiTokenEndpoint)
		if err != nil {
			break
		}

	}
	return err == nil
}
