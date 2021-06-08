package driver

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cnabio/cnab-go/bundle"
	"github.com/cnabio/cnab-go/bundle/definition"
	cnabdriver "github.com/cnabio/cnab-go/driver"
	"github.com/stretchr/testify/assert"

	"github.com/google/uuid"

	az "github.com/deislabs/cnab-azure-driver/pkg/azure"
	"github.com/deislabs/cnab-azure-driver/test"
)

var runAzureTest = flag.Bool("runazuretest", false, "Run tests in Azure")
var verboseDriver = flag.Bool("verbosedriveroutput", false, "Set Verbose Output in Azure Driver")

func TestNewACIDriver(t *testing.T) {

	testcases := []struct {
		name           string
		expectError    bool
		expectMessage  string
		envVarsToSet   map[string]string
		envVarsToUnset []string
		valuesToCheck  map[string]interface{}
	}{

		{"Either CNAB_AZURE_LOCATION or CNAB_AZURE_RESOURCE_GROUP must be set", true, "ACI Driver requires CNAB_AZURE_LOCATION environment variable or an existing Resource Group in CNAB_AZURE_RESOURCE_GROUP", map[string]string{}, []string{}, map[string]interface{}{}},
		{"No Error if CNAB_AZURE_LOCATION is set", false, "", map[string]string{"CNAB_AZURE_LOCATION": "test"}, []string{}, map[string]interface{}{"userAgent": "azure-cnab-driver-test-version", "aciLocation": "test"}},
		{"No Error if CNAB_AZURE_RESOURCE_GROUP is set", false, "", map[string]string{"CNAB_AZURE_RESOURCE_GROUP": "test"}, []string{"CNAB_AZURE_LOCATION"}, map[string]interface{}{"aciRG": "test"}},
		{"No Error if CNAB_AZURE_DELETE_RESOURCES is set", false, "", map[string]string{"CNAB_AZURE_DELETE_RESOURCES": "true"}, []string{}, map[string]interface{}{"deleteACIResources": true}},
		{"Both CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET should be set 1", true, "All of CNAB_AZURE_CLIENT_ID,CNAB_AZURE_CLIENT_SECRET must be set when one is set. CNAB_AZURE_CLIENT_SECRET is not set", map[string]string{"CNAB_AZURE_CLIENT_ID": "test"}, []string{}, map[string]interface{}{}},
		{"Both CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET should be set 1", true, "All of CNAB_AZURE_CLIENT_ID,CNAB_AZURE_CLIENT_SECRET must be set when one is set. CNAB_AZURE_CLIENT_ID is not set", map[string]string{"CNAB_AZURE_CLIENT_SECRET": "test"}, []string{"CNAB_AZURE_CLIENT_ID"}, map[string]interface{}{}},
		{"If CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET are set then CNAB_AZURE_TENANT_ID should be set", true, "CNAB_AZURE_TENANT_ID should be set when CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID are set", map[string]string{"CNAB_AZURE_CLIENT_ID": "test"}, []string{}, map[string]interface{}{}},
		{"Either CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID should be set not both", true, "either CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID should be set not both", map[string]string{"CNAB_AZURE_APP_ID": "test"}, []string{}, map[string]interface{}{}},
		{"No Error if CNAB_AZURE_CLIENT_ID, CNAB_AZURE_CLIENT_SECRET and CNAB_AZURE_TENANT_ID are set", false, "", map[string]string{"CNAB_AZURE_TENANT_ID": "test"}, []string{"CNAB_AZURE_APP_ID"}, map[string]interface{}{}},
		{"If CNAB_AZURE_TENANT_ID is set CNAB_AZURE_APP_ID or CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET should be set", true, "CNAB_AZURE_TENANT_ID should not be set when CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID are not set", map[string]string{}, []string{"CNAB_AZURE_CLIENT_ID", "CNAB_AZURE_CLIENT_SECRET"}, map[string]interface{}{}},
		{"No Error if CNAB_AZURE_TENANT_ID and CNAB_AZURE_APP_ID are set", false, "", map[string]string{"CNAB_AZURE_APP_ID": "test"}, []string{}, map[string]interface{}{}},
		{"If CNAB_AZURE_APP_ID is set CNAB_AZURE_TENANT_ID should be set", true, "CNAB_AZURE_TENANT_ID should be set when CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET or CNAB_AZURE_APP_ID are set", map[string]string{}, []string{"CNAB_AZURE_TENANT_ID"}, map[string]interface{}{}},
		{"No error when setting CNAB_AZURE_MSI_TYPE to system", false, "", map[string]string{"CNAB_AZURE_MSI_TYPE": "system"}, []string{"CNAB_AZURE_APP_ID"}, map[string]interface{}{"msiType": "system"}},
		{"CNAB_AZURE_USER_MSI_RESOURCE_ID must be set if user MSI is being used", true, "ACI Driver requires CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable when CNAB_AZURE_MSI_TYPE is set to user", map[string]string{"CNAB_AZURE_MSI_TYPE": "user"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_USER_MSI_RESOURCE_ID must be valid format", true, "CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable parsing error: parsing failed for invalid. Invalid resource Id format", map[string]string{"CNAB_AZURE_USER_MSI_RESOURCE_ID": "invalid"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_USER_MSI_RESOURCE_ID should be correct RP and Type", true, "CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.Storage/storageAccounts", map[string]string{"CNAB_AZURE_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.Storage/storageAccounts/name"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_USER_MSI_RESOURCE_ID should be correct Type", true, "CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.ManagedIdentity/storageAccounts", map[string]string{"CNAB_AZURE_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/storageAccounts/name"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_USER_MSI_RESOURCE_ID should be correct RP", true, "CNAB_AZURE_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.Storage/userAssignedIdentities", map[string]string{"CNAB_AZURE_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.Storage/userAssignedIdentities/name"}, []string{}, map[string]interface{}{}},
		{"No error when setting CNAB_AZURE_RESOURCE_GROUP", false, "", map[string]string{"CNAB_AZURE_USER_MSI_RESOURCE_ID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/userAssignedIdentities/name"}, []string{}, map[string]interface{}{"userMSIResourceID": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/userAssignedIdentities/name", "msiType": "user"}},
		{"No error when setting CNAB_AZURE_SUBSCRIPTION_ID", false, "", map[string]string{"CNAB_AZURE_SUBSCRIPTION_ID": "11111111-1111-1111-1111-111111111111"}, []string{}, map[string]interface{}{"subscriptionID": "11111111-1111-1111-1111-111111111111"}},
		{"CNAB_AZURE_REGISTRY_PASSWORD should be set if CNAB_AZURE_REGISTRY_USERNAME is set", true, "All of CNAB_AZURE_REGISTRY_USERNAME,CNAB_AZURE_REGISTRY_PASSWORD must be set when one is set. CNAB_AZURE_REGISTRY_PASSWORD is not set", map[string]string{"CNAB_AZURE_REGISTRY_USERNAME": "test"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_REGISTRY_USERNAME should be set if CNAB_AZURE_REGISTRY_PASSWORD is set", true, "All of CNAB_AZURE_REGISTRY_USERNAME,CNAB_AZURE_REGISTRY_PASSWORD must be set when one is set. CNAB_AZURE_REGISTRY_USERNAME is not set", map[string]string{"CNAB_AZURE_REGISTRY_PASSWORD": "test"}, []string{"CNAB_AZURE_REGISTRY_USERNAME"}, map[string]interface{}{}},
		{"No error when setting both CNAB_AZURE_REGISTRY_USERNAME and CNAB_AZURE_REGISTRY_PASSWORD", false, "", map[string]string{"CNAB_AZURE_REGISTRY_USERNAME": "test"}, []string{}, map[string]interface{}{"imageRegistryUser": "test", "imageRegistryPassword": "test"}},
		{"CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH should not be set if CNAB_AZURE_REGISTRY_USERNAME and CNAB_AZURE_REGISTRY_PASSWORD are set", true, "CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH should not be set if CNAB_AZURE_REGISTRY_USERNAME and CNAB_AZURE_REGISTRY_PASSWORD are set", map[string]string{"CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH": "true"}, []string{}, map[string]interface{}{}},
		{"Both CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET should be set when setting CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", true, "Both CNAB_AZURE_CLIENT_ID and CNAB_AZURE_CLIENT_SECRET should be set when setting CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", map[string]string{"CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH": "true"}, []string{"CNAB_AZURE_REGISTRY_USERNAME", "CNAB_AZURE_REGISTRY_PASSWORD"}, map[string]interface{}{}},
		{"CNAB_AZURE_CLIENT_SECRET should be set when setting CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", true, "All of CNAB_AZURE_CLIENT_ID,CNAB_AZURE_CLIENT_SECRET must be set when one is set. CNAB_AZURE_CLIENT_SECRET is not set", map[string]string{"CNAB_AZURE_CLIENT_ID": "test"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_CLIENT_ID should be set when setting CNAB_AZURE_USE_CLIENT_CREDS_FOR_REGISTRY_AUTH", true, "All of CNAB_AZURE_CLIENT_ID,CNAB_AZURE_CLIENT_SECRET must be set when one is set. CNAB_AZURE_CLIENT_ID is not set", map[string]string{"CNAB_AZURE_CLIENT_SECRET": "test"}, []string{"CNAB_AZURE_CLIENT_ID"}, map[string]interface{}{}},
		{"No error when setting CNAB_AZURE_CLIENT_CREDS_FOR_REGISTRY_AUTH", false, "", map[string]string{"CNAB_AZURE_CLIENT_ID": "test", "CNAB_AZURE_TENANT_ID": "test"}, []string{}, map[string]interface{}{"useSPForACR": true}},
		{"No error when setting CNAB_AZURE_PROPAGATE_CREDENTIALS", false, "", map[string]string{"CNAB_AZURE_PROPAGATE_CREDENTIALS": "true"}, []string{}, map[string]interface{}{"propagateCredentials": true}},
		{"CNAB_AZURE_STATE_ options should all be set 1", true, "All of CNAB_AZURE_STATE_FILESHARE,CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME,CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY must be set when one is set. CNAB_AZURE_STATE_FILESHARE is not set", map[string]string{"CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME": "test", "CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY": "test"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_STATE_ options should all be set 2", true, "All of CNAB_AZURE_STATE_FILESHARE,CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME,CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY must be set when one is set. CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME is not set", map[string]string{"CNAB_AZURE_STATE_FILESHARE": "test"}, []string{"CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME"}, map[string]interface{}{}},
		{"CNAB_AZURE_STATE_ options should all be set 3", true, "All of CNAB_AZURE_STATE_FILESHARE,CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME,CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY must be set when one is set. CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY is not set", map[string]string{"CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME": "test"}, []string{"CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY"}, map[string]interface{}{}},
		{"CNAB_AZURE_STATE_MOUNT_POINT_should_be_an_absolute_path", true, "value (test) of CNAB_AZURE_STATE_MOUNT_POINT is not an absolute path", map[string]string{"CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY": "test", "CNAB_AZURE_STATE_MOUNT_POINT": "test"}, []string{}, map[string]interface{}{}},
		{"CNAB_AZURE_STATE_MOUNT_POINT_should_not be root path", true, "CNAB_AZURE_STATE_MOUNT_POINT should not be root path", map[string]string{"CNAB_AZURE_STATE_MOUNT_POINT": "/../"}, []string{}, map[string]interface{}{}},
		{"No error when setting CNAB_AZURE_MOUNT_POINT", false, "", map[string]string{"CNAB_AZURE_STATE_MOUNT_POINT": "/mnt/path"}, []string{}, map[string]interface{}{"mountStateVolume": true, "stateMountPoint": "/mnt/path"}},
		//{"No error when setting CNAB_AZURE_STATE_PATH", false, "", map[string]string{"CNAB_AZURE_STATE_PATH": "/statepath"}, []string{}, map[string]interface{}{"mountStateVolume": true, "statePath": "/statepath"}},
	}
	// Unset any CNAB_AZURE environment variables as these will make the tests fail
	test.UnSetDriverEnvironmentVars(t)
	defer test.UnSetDriverEnvironmentVars(t)

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
						assert.Equal(t, v, test.GetFieldValue(t, d, k))
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
}
func TestCanWriteOutputs(t *testing.T) {
	os.Setenv("CNAB_AZURE_LOCATION", "test")
	defer test.UnSetDriverEnvironmentVars(t)
	op := cnabdriver.Operation{
		Action:       "install",
		Installation: "test",
		Parameters: map[string]interface{}{
			"param1": "value1",
			"param2": "value2",
		},
		Image: bundle.InvocationImage{
			BaseImage: bundle.BaseImage{
				Image:     "simongdavies/helloworld-aci-cnab",
				ImageType: "docker",
				Digest:    "sha256:ba27c336615454378b0c1d85ef048583b1fd607b1a96defc90988292e9fb1edb",
			},
		},
		Bundle: &bundle.Bundle{
			Definitions: definition.Definitions{
				"output1": &definition.Schema{},
				"output2": &definition.Schema{},
			},
			Outputs: map[string]bundle.Output{
				"output1": {
					Definition: "output1",
					Path:       "/cnab/app/outputs/output1",
				},
				"output2": {
					Definition: "output2",
					Path:       "/cnab/app/outputs/output2",
				},
			},
		},
		Revision: "01DDY0MT808KX0GGZ6SMXN4TW",
		Environment: map[string]string{
			"ENV1": "value1",
			"ENV2": "value2",
		},
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
		Outputs: map[string]string{
			"output1": "/cnab/app/outputs/output1",
			"output2": "/cnab/app/outputs/output2",
		},
	}

	d, err := NewACIDriver("test-version")
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation. Got: %v", err)
	assert.NotNil(t, d)
	_, err = d.Run(&op)
	assert.Error(t, err, "Bundle has outputs no volume mounted for state, set CNAB_AZURE_STATE_* variables so that state can be retrieved")

}
func TestRunAzureTest(t *testing.T) {

	if !*runAzureTest {
		t.Skip("Not running tests in Azure")
	}

	test.UnSetDriverEnvironmentVars(t)
	// Set environments vars using TEST_ to configure the driver before running the test, if these are not set the the driver tries to login using the cloudshell or az cli
	loginEnvVars := []string{
		"CNAB_AZURE_SUBSCRIPTION_ID",
		"CNAB_AZURE_CLIENT_SECRET",
		"CNAB_AZURE_LOCATION",
		"CNAB_AZURE_CLIENT_ID",
		"CNAB_AZURE_TENANT_ID"}

	// Check for environment variables to use for login these are expected to be the name of the relevant driver variable prefixed with TEST_
	for _, e := range loginEnvVars {
		envvar := os.Getenv(fmt.Sprintf("TEST_%s", e))
		t.Logf("Setting Env Variable: %s", e)
		os.Setenv(e, envvar)
	}
	defer test.UnSetDriverEnvironmentVars(t)

	// Set verbose output for the driver
	test.SetLoggingLevel(verboseDriver)

	// Set a default location if not set
	envvar := os.Getenv("CNAB_AZURE_LOCATION")
	if len(envvar) == 0 {
		os.Setenv("CNAB_AZURE_LOCATION", "westeurope")
	}

	op := cnabdriver.Operation{
		Action:       "install",
		Installation: "test-install",
		Parameters:   map[string]interface{}{},
		Image: bundle.InvocationImage{
			BaseImage: bundle.BaseImage{
				Image:     "simongdavies/helloworld-aci-cnab",
				ImageType: "docker",
				Digest:    "sha256:a9137fc4cb1d3c79533a45bbaa437d6f45e501a61b9c882a1ca4960fafe0ae3c",
			},
		},
		Environment: map[string]string{
			"CNAB_INSTALLATION_NAME": "test-aci",
			"CNAB_ACTION":            "install",
			"CNAB_BUNDLE_NAME":       "helloworld-aci",
			"CNAB_BUNDLE_VERSION":    "0.1.0",
			"ENV1":                   "value1",
			"ENV2":                   "value2",
		},
		Revision: "01DDY0MT808KX0GGZ6SMXN4TW",
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
	}

	d, err := NewACIDriver("test-version")
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation. Got: %v", err)
	assert.NotNil(t, d)
	_, err = d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation. Got: %v", err)

	// Test image reference with tag and digest

	op = cnabdriver.Operation{
		Action:       "install",
		Installation: "test-install",
		Parameters:   map[string]interface{}{},
		Image: bundle.InvocationImage{
			BaseImage: bundle.BaseImage{
				Image:     "simongdavies/helloworld-aci-cnab:latest",
				ImageType: "docker",
				Digest:    "sha256:a9137fc4cb1d3c79533a45bbaa437d6f45e501a61b9c882a1ca4960fafe0ae3c",
			},
		},
		Environment: map[string]string{
			"CNAB_INSTALLATION_NAME": "test-aci",
			"CNAB_ACTION":            "install",
			"CNAB_BUNDLE_NAME":       "helloworld-aci",
			"CNAB_BUNDLE_VERSION":    "0.1.0",
			"ENV1":                   "value1",
			"ENV2":                   "value2",
		},
		Revision: "01DDY0MT808KX0GGZ6SMXN4TW",
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
	}

	d, err = NewACIDriver("test-version")
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation. Got: %v", err)
	assert.NotNil(t, d)
	_, err = d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation. Got: %v", err)

	// Test op with files

	op = cnabdriver.Operation{
		Action:       "install",
		Installation: "test-install-with-files",
		Parameters:   map[string]interface{}{},
		Image: bundle.InvocationImage{
			BaseImage: bundle.BaseImage{
				Image:     "simongdavies/helloworld-aci-cnab",
				ImageType: "docker",
				Digest:    "sha256:a9137fc4cb1d3c79533a45bbaa437d6f45e501a61b9c882a1ca4960fafe0ae3c",
			},
		},
		Revision: "01DDY0MT808KX0GGZ6SMXN4TW",
		Environment: map[string]string{
			"CNAB_INSTALLATION_NAME": "test-install-with-files",
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
	_, err = d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation with files. Got: %v", err)

	// Test Mounting Storage

	fileShareEnvVars := []string{
		"CNAB_AZURE_STATE_FILESHARE",
		"CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME",
		"CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY",
	}

	for _, e := range fileShareEnvVars {
		envvarName := fmt.Sprintf("TEST_%s", e)
		envvar := os.Getenv(envvarName)
		if len(envvar) == 0 {
			t.Logf("Environment Variable %s is not set", envvarName)
			t.FailNow()
		}
		t.Logf("Setting Env Variable: %s", e)
		os.Setenv(e, envvar)
	}
	test.SetStatePathEnvironmentVariables()
	op = cnabdriver.Operation{
		Action: "install",
		Bundle: &bundle.Bundle{
			Name: "test-install-with-state",
		},
		Installation: "",
		Image: bundle.InvocationImage{
			BaseImage: bundle.BaseImage{
				Image:     "simongdavies/azure-outputs-cnab",
				ImageType: "docker",
				Digest:    "sha256:9613017ac6738d7fce618987c293991cae9f996f8dd62c23fc4065580bbd3476",
			},
		},
		Revision: "01DDY0MT808KX0GGZ6SMXN4TW",
		Environment: map[string]string{
			"CNAB_INSTALLATION_NAME": "test-install-with-state",
			"CNAB_ACTION":            "install",
		},
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
	}
	op.Installation = uuid.New().String()
	d, err = NewACIDriver("test-version")
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation with mounted state storage. Got: %v", err)
	assert.NotNil(t, d)
	_, err = d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation with mounted state storage. Got: %v", err)
	afs, err := az.NewFileShare(os.Getenv("TEST_CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME"), os.Getenv("TEST_CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY"), os.Getenv("TEST_CNAB_AZURE_STATE_FILESHARE"))
	assert.NoErrorf(t, err, "Expected no error when creating FileShare object. Got: %v", err)
	// Check State was written
	statePath := fmt.Sprintf("%s/%s", strings.ToLower(op.Bundle.Name), strings.ToLower(op.Installation))
	content, err := afs.ReadFileFromShare(statePath + "/teststate")
	assert.NoErrorf(t, err, "Expected no error when reading state. Got: %v", err)
	assert.EqualValuesf(t, "TEST", content, "Expected state to be TEST but got %s", content)

	// Test Outputs

	op = cnabdriver.Operation{
		Action:       "install",
		Installation: "",
		Image: bundle.InvocationImage{
			BaseImage: bundle.BaseImage{
				Image:     "simongdavies/azure-outputs-cnab",
				ImageType: "docker",
				Digest:    "sha256:6abd5787989b6303b91fee441e351829bd3921601ffcb390681884ee49a3a38f",
			},
		},
		Bundle: &bundle.Bundle{
			Name: "test-install-with-outputs",
			Definitions: definition.Definitions{
				"output1": &definition.Schema{},
				"output2": &definition.Schema{},
			},
			Outputs: map[string]bundle.Output{
				"output1": {
					Definition: "output1",
					Path:       "/cnab/app/outputs/output1",
				},
				"output2": {
					Definition: "output2",
					Path:       "/cnab/app/outputs/output2",
				},
			},
		},
		Revision: "01DDY0MT808KX0GGZ6SMXN4TW",
		Environment: map[string]string{
			"CNAB_INSTALLATION_NAME": "test-install-with-outputs",
			"CNAB_ACTION":            "install",
			"CNAB_BUNDLE_NAME":       "helloworld-aci",
			"CNAB_BUNDLE_VERSION":    "0.1.0",
		},
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
		Outputs: map[string]string{
			"/cnab/app/outputs/output1": "output1",
			"/cnab/app/outputs/output2": "output2",
		},
	}
	op.Installation = uuid.New().String()
	d, err = NewACIDriver("test-version")
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation with outputs. Got: %v", err)
	assert.NotNil(t, d)
	results, err := d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation with outputs. Got: %v", err)
	outputs := getOutputs(results)
	assert.EqualValuesf(t, 2, len(outputs), "Expected to get 2 outputs when running Test Operation with outputs but got %d", len(outputs))
	if len(outputs) == 2 {
		assert.Equal(t, map[string]string{
			"/cnab/app/outputs/output1": "OUTPUT_1",
			"/cnab/app/outputs/output2": "OUTPUT_2",
		}, results.Outputs)
	}
}

func TestValidateMSIScope(t *testing.T) {
	testcases := []struct {
		name        string
		scope       string
		expectError bool
	}{
		{
			name:        "valid subscription scope",
			scope:       "/subscriptions/f1fc7a2d-ce60-43ea-b6af-808d7896eba8",
			expectError: false,
		},
		{
			name:        "valid resource group scope",
			scope:       "/subscriptions/28fb4867-4cef-43ed-9637-b678cd6b2ce3/resourceGroups/myvalidrg",
			expectError: false,
		},
		{
			name:        "valid resource scope",
			scope:       "/subscriptions/995c6481-7ddc-4dac-afd4-33545d96cf29/resourceGroups/myvalidrg/providers/Microsoft.Web/serverFarms/myvalidweb",
			expectError: false,
		},
		{
			name:        "valid resource group scope containing period",
			scope:       "/subscriptions/96f5b6e2-d2a8-4bdd-9705-ee9b5c8ca44e/resourceGroups/my.valid.rg",
			expectError: false,
		},
		{
			name:        "invalid subscription scope with missing leading slash",
			scope:       "subscriptions/995c6481-7ddc-4dac-afd4-33545d96cf29",
			expectError: true,
		},
		{
			name:        "invalid subscription scope with trailing slash",
			scope:       "/subscriptions/995c6481-7ddc-4dac-afd4-33545d96cf29/",
			expectError: true,
		},
		{
			name:        "invalid subscription scope with none guid subscription id",
			scope:       "/subscriptions/mysubscriptionid",
			expectError: true,
		},
		{
			name:        "invalid subscription scope with wrong casing",
			scope:       "/Subscriptions/40690205-8c01-4798-8788-fd132af03032",
			expectError: true,
		},
		{
			name:        "invalid subscription scope with wrong guid length",
			scope:       "/subscriptions/3a4a5acf-4f1c-4dff-92e8-df61a5fcca",
			expectError: true,
		},
		{
			name:        "invalid subscription scope with wrong guid format",
			scope:       "/subscriptions/{880f0f9e-39a4-4092-ba31-c826e42534c0}",
			expectError: true,
		},
		{
			name:        "invalid resource group scope with trailing period",
			scope:       "/subscriptions/28fb4867-4cef-43ed-9637-b678cd6b2ce3/resourceGroups/myinvalidrg.",
			expectError: true,
		},
		{
			name:        "invalid resource group scope wrong casing",
			scope:       "/subscriptions/55e3c99a-5225-40d4-b117-80b30b2847e8/ResourceGroups/myinvalidrg",
			expectError: true,
		},
		{
			name:        "invalid resource group scope with extra slash",
			scope:       "/subscriptions/adc8cb68-a92d-4e01-8d8f-b117b3571ef2/ResourceGroups/dev/myinvalidrg",
			expectError: true,
		},
		{
			name:        "invalid resource group scope with trailing slash",
			scope:       "/subscriptions/f4390b3c-4184-4cf7-9454-5e273adc6d2b/ResourceGroups/myinvalidrg/",
			expectError: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMSIScope(tc.scope)
			if tc.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func getOutputs(results cnabdriver.OperationResult) (outputs []string) {
	for _, item := range results.Outputs {
		outputs = append(outputs, item)
	}
	return outputs
}
