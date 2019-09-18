package driver

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/stretchr/testify/assert"
)

var runazuretest = flag.Bool("runazuretest", false, "Run tests in Azure")

func TestNewACIDriver(t *testing.T) {

	testcases := []struct {
		name           string
		expectError    bool
		expectMessage  string
		envVarsToSet   map[string]string
		envVarsToUnset []string
		valuesToCheck  map[string]interface{}
	}{

		{"Either DUFFLE_ACI_DRIVER_LOCATION or DUFFLE_ACI_DRIVER_RESOURCE_GROUP must be set", true, "ACI Driver requires DUFFLE_ACI_DRIVER_LOCATION environment variable or an existing Resource Group in DUFFLE_ACI_DRIVER_RESOURCE_GROUP", map[string]string{}, []string{}, map[string]interface{}{}},
		{"No Error if DUFFLE_ACI_DRIVER_LOCATION is set", false, "", map[string]string{"DUFFLE_ACI_DRIVER_LOCATION": "test"}, []string{}, map[string]interface{}{"userAgent": "DuffleACIDriver-test-version", "aciLocation": "test"}},
		{"No Error if DUFFLE_ACI_DRIVER_RESOURCE_GROUP is set", false, "", map[string]string{"DUFFLE_ACI_DRIVER_RESOURCE_GROUP": "test"}, []string{"DUFFLE_ACI_DRIVER_LOCATION"}, map[string]interface{}{"aciRG": "test"}},
		{"No Error if DUFFLE_ACI_DRIVER_DELETE_RESOURCES is set", false, "", map[string]string{"DUFFLE_ACI_DRIVER_DELETE_RESOURCES": "true"}, []string{}, map[string]interface{}{"deleteACIResources": true}},
		{"Both DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET should be set 1", true, "All of DUFFLE_ACI_DRIVER_CLIENT_ID,DUFFLE_ACI_DRIVER_CLIENT_SECRET must be set when one is set. DUFFLE_ACI_DRIVER_CLIENT_SECRET is not set", map[string]string{"DUFFLE_ACI_DRIVER_CLIENT_ID": "test"}, []string{}, map[string]interface{}{}},
		{"Both DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET should be set 1", true, "All of DUFFLE_ACI_DRIVER_CLIENT_ID,DUFFLE_ACI_DRIVER_CLIENT_SECRET must be set when one is set. DUFFLE_ACI_DRIVER_CLIENT_ID is not set", map[string]string{"DUFFLE_ACI_DRIVER_CLIENT_SECRET": "test"}, []string{"DUFFLE_ACI_DRIVER_CLIENT_ID"}, map[string]interface{}{}},
		{"If DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET are set then DUFFLE_ACI_DRIVER_TENANT_ID should be set", true, "DUFFLE_ACI_DRIVER_TENANT_ID should be set when DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID are set", map[string]string{"DUFFLE_ACI_DRIVER_CLIENT_ID": "test"}, []string{}, map[string]interface{}{}},
		{"Either DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID should be set not both", true, "either DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID should be set not both", map[string]string{"DUFFLE_ACI_DRIVER_APP_ID": "test"}, []string{}, map[string]interface{}{}},
		{"No Error if DUFFLE_ACI_DRIVER_CLIENT_ID, DUFFLE_ACI_DRIVER_CLIENT_SECRET and DUFFLE_ACI_DRIVER_TENANT_ID are set", false, "", map[string]string{"DUFFLE_ACI_DRIVER_TENANT_ID": "test"}, []string{"DUFFLE_ACI_DRIVER_APP_ID"}, map[string]interface{}{}},
		{"If DUFFLE_ACI_DRIVER_TENANT_ID is set DUFFLE_ACI_DRIVER_APP_ID or DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET should be set", true, "DUFFLE_ACI_DRIVER_TENANT_ID should not be set when DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID are not set", map[string]string{}, []string{"DUFFLE_ACI_DRIVER_CLIENT_ID", "DUFFLE_ACI_DRIVER_CLIENT_SECRET"}, map[string]interface{}{}},
		{"No Error if DUFFLE_ACI_DRIVER_TENANT_ID and DUFFLE_ACI_DRIVER_APP_ID are set", false, "", map[string]string{"DUFFLE_ACI_DRIVER_APP_ID": "test"}, []string{}, map[string]interface{}{}},
		{"If DUFFLE_ACI_DRIVER_APP_ID is set DUFFLE_ACI_DRIVER_TENANT_ID should be set", true, "DUFFLE_ACI_DRIVER_TENANT_ID should be set when DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET or DUFFLE_ACI_DRIVER_APP_ID are set", map[string]string{}, []string{"DUFFLE_ACI_DRIVER_TENANT_ID"}, map[string]interface{}{}},
		{"No error when setting DUFFLE_ACI_DRIVER_MSI_TYPE to system", false, "", map[string]string{"DUFFLE_ACI_DRIVER_MSI_TYPE": "system"}, []string{"DUFFLE_ACI_DRIVER_APP_ID"}, map[string]interface{}{"msiType": "system"}},
		{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID must be set if user MSI is being used", true, "ACI Driver requires DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable when DUFFLE_ACI_DRIVER_MSI_TYPE is set to user", map[string]string{"DUFFLE_ACI_DRIVER_MSI_TYPE": "user"}, []string{}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID must be valid format", true, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable parsing error: parsing failed for invalid. Invalid resource Id format", map[string]string{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID": "invalid"}, []string{}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID should be correct RP and Type", true, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.Storage/storageAccounts", map[string]string{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.Storage/storageAccounts/name"}, []string{}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID should be correct Type", true, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.ManagedIdentity/storageAccounts", map[string]string{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/storageAccounts/name"}, []string{}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID should be correct RP", true, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.Storage/userAssignedIdentities", map[string]string{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.Storage/userAssignedIdentities/name"}, []string{}, map[string]interface{}{}},
		{"No error when setting DUFFLE_ACI_DRIVER_RESOURCE_GROUP", false, "", map[string]string{"DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/userAssignedIdentities/name"}, []string{}, map[string]interface{}{"userMSIResourceID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/userAssignedIdentities/name", "msiType": "user"}},
		{"No error when setting DUFFLE_ACI_DRIVER_SUBSCRIPTION_ID", false, "", map[string]string{"DUFFLE_ACI_DRIVER_SUBSCRIPTION_ID": "11111111-1111-1111-1111-111111111111"}, []string{}, map[string]interface{}{"subscriptionID": "11111111-1111-1111-1111-111111111111"}},
		{"DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD should be set if DUFFLE_ACI_DRIVER_REGISTRY_USERNAME is set", true, "All of DUFFLE_ACI_DRIVER_REGISTRY_USERNAME,DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD must be set when one is set. DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD is not set", map[string]string{"DUFFLE_ACI_DRIVER_REGISTRY_USERNAME": "test"}, []string{}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_REGISTRY_USERNAME should be set if DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD is set", true, "All of DUFFLE_ACI_DRIVER_REGISTRY_USERNAME,DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD must be set when one is set. DUFFLE_ACI_DRIVER_REGISTRY_USERNAME is not set", map[string]string{"DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD": "test"}, []string{"DUFFLE_ACI_DRIVER_REGISTRY_USERNAME"}, map[string]interface{}{}},
		{"No error when setting both DUFFLE_ACI_DRIVER_REGISTRY_USERNAME and DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD", false, "", map[string]string{"DUFFLE_ACI_DRIVER_REGISTRY_USERNAME": "test"}, []string{}, map[string]interface{}{"imageRegistryUser": "test", "imageRegistryPassword": "test"}},
		{"DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH should not be set if DUFFLE_ACI_DRIVER_REGISTRY_USERNAME and DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD are set", true, "DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH should not be set if DUFFLE_ACI_DRIVER_REGISTRY_USERNAME and DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD are set", map[string]string{"DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH": "true"}, []string{}, map[string]interface{}{}},
		{"Both DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET should be set when setting DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", true, "Both DUFFLE_ACI_DRIVER_CLIENT_ID and DUFFLE_ACI_DRIVER_CLIENT_SECRET should be set when setting DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", map[string]string{"DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH": "true"}, []string{"DUFFLE_ACI_DRIVER_REGISTRY_USERNAME", "DUFFLE_ACI_DRIVER_REGISTRY_PASSWORD"}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_CLIENT_SECRET should be set when setting DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", true, "All of DUFFLE_ACI_DRIVER_CLIENT_ID,DUFFLE_ACI_DRIVER_CLIENT_SECRET must be set when one is set. DUFFLE_ACI_DRIVER_CLIENT_SECRET is not set", map[string]string{"DUFFLE_ACI_DRIVER_CLIENT_ID": "test"}, []string{}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_CLIENT_ID should be set when setting DUFFLE_ACI_DRIVER_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", true, "All of DUFFLE_ACI_DRIVER_CLIENT_ID,DUFFLE_ACI_DRIVER_CLIENT_SECRET must be set when one is set. DUFFLE_ACI_DRIVER_CLIENT_ID is not set", map[string]string{"DUFFLE_ACI_DRIVER_CLIENT_SECRET": "test"}, []string{"DUFFLE_ACI_DRIVER_CLIENT_ID"}, map[string]interface{}{}},
		{"No error when setting DUFFLE_ACI_DRIVER_CLIENT_CREDS_FOR_REGISTRY_AUTH", false, "", map[string]string{"DUFFLE_ACI_DRIVER_CLIENT_ID": "test", "DUFFLE_ACI_DRIVER_TENANT_ID": "test"}, []string{}, map[string]interface{}{"useSPForACR": true}},
		{"No error when setting DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS", false, "", map[string]string{"DUFFLE_ACI_DRIVER_PROPAGATE_CREDENTIALS": "true"}, []string{}, map[string]interface{}{"propagateCredentials": true}},
		{"DUFFLE_ACI_DRIVER_STATE_ options should all be set 1", true, "All of DUFFLE_ACI_DRIVER_STATE_PATH,DUFFLE_ACI_DRIVER_STATE_FILESHARE,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY,DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT must be set when one is set. DUFFLE_ACI_DRIVER_STATE_FILESHARE is not set", map[string]string{"DUFFLE_ACI_DRIVER_STATE_PATH": "test", "DUFFLE_ACI_DRIVER_MOUNT_POINT": "test", "DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME": "test", "DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY": "test"}, []string{}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_STATE_ options should all be set 2", true, "All of DUFFLE_ACI_DRIVER_STATE_PATH,DUFFLE_ACI_DRIVER_STATE_FILESHARE,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY,DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT must be set when one is set. DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME is not set", map[string]string{"DUFFLE_ACI_DRIVER_STATE_FILESHARE": "test"}, []string{"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME"}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_STATE_ options should all be set 3", true, "All of DUFFLE_ACI_DRIVER_STATE_PATH,DUFFLE_ACI_DRIVER_STATE_FILESHARE,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY,DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT must be set when one is set. DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY is not set", map[string]string{"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME": "test"}, []string{"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY"}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_STATE_ options should all be set 4", true, "All of DUFFLE_ACI_DRIVER_STATE_PATH,DUFFLE_ACI_DRIVER_STATE_FILESHARE,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY,DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT must be set when one is set. DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT is not set", map[string]string{"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY": "test"}, []string{"DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT"}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_STATE_ options should all be set 5", true, "All of DUFFLE_ACI_DRIVER_STATE_PATH,DUFFLE_ACI_DRIVER_STATE_FILESHARE,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME,DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY,DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT must be set when one is set. DUFFLE_ACI_DRIVER_STATE_PATH is not set", map[string]string{"DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT": "test"}, []string{"DUFFLE_ACI_DRIVER_STATE_PATH"}, map[string]interface{}{}},
		{"DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT_should_be_an_absolute_path", true, "value (test) of DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT is not an absolute path", map[string]string{"DUFFLE_ACI_DRIVER_STATE_PATH": "test", "DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT": "test"}, []string{}, map[string]interface{}{}},
		{"No error when setting DUFFLE_ACI_DRIVER__MOUNT_POINT", false, "", map[string]string{"DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT": "/mnt/path"}, []string{"DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT"}, map[string]interface{}{"mountStateVolume": true}},
	}
	// Unset any DUFFLE_ACI_DRIVER environment variables as these will make the tests fail
	unSetDriverEnvironmentVars(t)

	for _, tc := range testcases {
		for _, n := range tc.envVarsToUnset {
			os.Unsetenv(n)
		}
		for k, v := range tc.envVarsToSet {
			os.Setenv(k, v)
		}
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewACIDriver("test-version")
			if tc.expectError {
				assert.EqualError(t, err, tc.expectMessage)
				assert.Nil(t, d)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, d)
				if d != nil {
					for k, v := range tc.valuesToCheck {
						assert.Equal(t, v, getFieldValue(t, d, k))
					}
				}
			}

		})
	}

	// The driver should handle Docker and OCI invocation images
	d, _ := NewACIDriver("test-version")
	assert.Equal(t, true, d.Handles(cnabdriver.ImageTypeDocker))
	assert.Equal(t, true, d.Handles(cnabdriver.ImageTypeOCI))
	assert.Equal(t, false, d.Handles(cnabdriver.ImageTypeQCOW))

	unSetDriverEnvironmentVars(t)
}

func TestRunAzureTest(t *testing.T) {

	if !*runazuretest {
		t.Skip("Not running tests in Azure")
	}
	unSetDriverEnvironmentVars(t)
	// Set environments vars using TEST_ to configure the driver before running the test, if these are not set the the driver tries to login using the cloudshell or az cli
	loginEnvVars := []string{
		"DUFFLE_ACI_DRIVER_SUBSCRIPTION_ID",
		"DUFFLE_ACI_DRIVER_CLIENT_SECRET",
		"DUFFLE_ACI_DRIVER_LOCATION",
		"DUFFLE_ACI_DRIVER_CLIENT_ID",
		"DUFFLE_ACI_DRIVER_TENANT_ID"}

	// Check for environment variables to use for login these are expected to be the name of the relevant driver variable prefixed with TEST_
	for _, e := range loginEnvVars {
		envvar := os.Getenv(fmt.Sprintf("TEST_%s", e))
		t.Logf("Setting Env Variable: %s=%s", e, envvar)
		os.Setenv(e, envvar)
	}
	defer unSetDriverEnvironmentVars(t)
	// defer func() {
	// 	for _, e := range os.Environ() {
	// 		pair := strings.Split(e, "=")
	// 		if strings.HasPrefix(pair[0], "DUFFLE_ACI_DRIVER") {
	// 			t.Logf("Unsetting Env Variable: %s", pair[0])
	// 			os.Unsetenv(pair[0])
	// 		}
	// 	}
	// }()

	// Set verbose output for the driver
	os.Setenv("DUFFLE_ACI_DRIVER_VERBOSE", "true")
	// Set a default location if not set
	envvar := os.Getenv("DUFFLE_ACI_DRIVER_LOCATION")
	if len(envvar) == 0 {
		os.Setenv("DUFFLE_ACI_DRIVER_LOCATION", "westeurope")
	}

	op := cnabdriver.Operation{
		Action:       "install",
		Installation: "test",
		Parameters:   map[string]interface{}{},
		Revision:     "01DDY0MT808KX0GGZ6SMXN4TW",
		Image:        "simongdavies/helloworld-aci-cnab:c25f7f06fbc608e7bcfabd7e2700c5976e824286",
		ImageType:    "docker",
		Environment: map[string]string{
			"CNAB_INSTALLATION_NAME": "test-aci",
			"CNAB_ACTION":            "install",
			"CNAB_BUNDLE_NAME":       "helloworld-aci",
			"CNAB_BUNDLE_VERSION":    "0.1.0",
		},
	}
	d, err := NewACIDriver("test-version")
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation. Got: %v", err)
	assert.NotNil(t, d)
	err = d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation. Got: %v", err)

	// Test op with files

	op = cnabdriver.Operation{
		Action:       "install",
		Installation: "test",
		Parameters:   map[string]interface{}{},
		Revision:     "01DDY0MT808KX0GGZ6SMXN4TW",
		Image:        "simongdavies/helloworld-aci-cnab:c25f7f06fbc608e7bcfabd7e2700c5976e824286",
		ImageType:    "docker",
		Environment: map[string]string{
			"CNAB_INSTALLATION_NAME": "test-aci",
			"CNAB_ACTION":            "install",
			"CNAB_BUNDLE_NAME":       "helloworld-aci",
			"CNAB_BUNDLE_VERSION":    "0.1.0",
		},
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
			"/tmp/test":                "testcontent",
		},
	}
	d, err = NewACIDriver("test-version")
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation with files. Got: %v", err)
	assert.NotNil(t, d)
	err = d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation with files. Got: %v", err)

	// Test Mounting State FileShare

	fileShareEnvVars := []string{
		"DUFFLE_ACI_DRIVER_STATE_FILESHARE",
		"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_NAME",
		"DUFFLE_ACI_DRIVER_STATE_STORAGE_ACCOUNT_KEY",
		"DUFFLE_ACI_DRIVER_STATE_PATH",
		"DUFFLE_ACI_DRIVER_STATE_MOUNT_POINT"}

	for _, e := range fileShareEnvVars {
		envvar := os.Getenv(fmt.Sprintf("TEST_%s", e))
		t.Logf("Setting Env Variable: %s=%s", e, envvar)
		os.Setenv(e, envvar)
	}

	unSetDriverEnvironmentVars(t)
}

func unSetDriverEnvironmentVars(t *testing.T) {
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		if strings.HasPrefix(pair[0], "DUFFLE_ACI_DRIVER") {
			t.Logf("Unsetting Env Variable: %s", pair[0])
			os.Unsetenv(pair[0])
		}
	}
}

func getFieldValue(t *testing.T, driver cnabdriver.Driver, field string) interface{} {
	r := reflect.ValueOf(driver)
	f := reflect.Indirect(r).FieldByName(field)
	if f.IsValid() {
		switch f.Kind() {
		case reflect.String:
			return f.String()
		case reflect.Int:
			return f.Int()
		case reflect.Bool:
			return f.Bool()
		default:
			t.Errorf("field %s has unexpected type %s ", field, f.Kind())
			return nil
		}
	}

	t.Errorf("Unable to get value for field %s ", field)
	return nil

}
