package driver

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/stretchr/testify/assert"
)

var runazuretest = flag.Bool("runazuretest", false, "Run tests in Azure")

func TestNewACIDriver(t *testing.T) {

	// Unset any DUFFLE_ACI_DRIVER environment variables as these will make the tests fail
	unSetDriverEnvironmentVars(t)

	// Either DUFFLE_ACI_DRIVER_LOCATION = DUFFLE_ACI_DRIER_RESOURCE_GROUP must be set
	d, err := NewACIDriver()
	assert.EqualError(t, err, "ACI Driver requires DUFFLE_ACI_DRIVER_LOCATION environment variable or an existing Resource Group in DUFFLE_ACI_DRIVER_RESOURCE_GROUP")
	assert.Nil(t, d)

	os.Setenv("DUFFLE_ACI_DRIVER_LOCATION", "test")
	d, err = NewACIDriver()
	assert.NoErrorf(t, err, "Expected no error when setting DUFFLE_ACI_DRIVER_LOCATION got: %v", err)
	assert.NotNil(t, d)

	os.Unsetenv("DUFFLE_ACI_DRIVER_LOCATION")
	os.Setenv("DUFFLE_ACI_DRIVER_RESOURCE_GROUP", "test")
	d, err = NewACIDriver()
	assert.NoErrorf(t, err, "Expected no error when setting DUFFLE_ACI_DRIVER_RESOURCE_GROUP got: %v", err)
	assert.NotNil(t, d)

	// If DUFFLE_ACI_DRIVER_MSI_TYPE is specified it must be user or system
	os.Setenv("DUFFLE_ACI_DRIVER_MSI_TYPE", "invalid")
	d, err = NewACIDriver()
	assert.EqualError(t, err, "DUFFLE_ACI_DRIVER_MSI_TYPE environment variable unknown value: invalid")
	assert.Nil(t, d)

	os.Setenv("DUFFLE_ACI_DRIVER_MSI_TYPE", "system")
	d, err = NewACIDriver()
	assert.NoErrorf(t, err, "Expected no error when setting DUFFLE_ACI_DRIVER_MSI_TYPE to system got: %v", err)
	assert.NotNil(t, d)

	// DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID must be set if user MSI is being used
	os.Setenv("DUFFLE_ACI_DRIVER_MSI_TYPE", "user")
	d, err = NewACIDriver()
	assert.EqualError(t, err, "ACI Driver requires DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable when DUFFLE_ACI_DRIVER_MSI_TYPE is set to user")
	assert.Nil(t, d)

	os.Setenv("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID", "invalid")
	d, err = NewACIDriver()
	assert.EqualError(t, err, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable parsing error: parsing failed for invalid. Invalid resource Id format")
	assert.Nil(t, d)

	os.Setenv("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID", "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.Storage/storageAccounts/name")
	d, err = NewACIDriver()
	assert.EqualError(t, err, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.Storage/storageAccounts")
	assert.Nil(t, d)

	os.Setenv("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID", "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/storageAccounts/name")
	d, err = NewACIDriver()
	assert.EqualError(t, err, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.ManagedIdentity/storageAccounts")
	assert.Nil(t, d)

	os.Setenv("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID", "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.Storage/userAssignedIdentities/name")
	d, err = NewACIDriver()
	assert.EqualError(t, err, "DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID environment variable RP type should be Microsoft.ManagedIdentity/userAssignedIdentities got: Microsoft.Storage/userAssignedIdentities")
	assert.Nil(t, d)

	os.Setenv("DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID", "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/name/providers/Microsoft.ManagedIdentity/userAssignedIdentities/name")
	d, err = NewACIDriver()
	assert.NoErrorf(t, err, "Expected no error when setting DUFFLE_ACI_DRIVER_RESOURCE_GROUP got: %v", err)
	assert.NotNil(t, d)

	// The driver should handle Docker and OCI invocation images
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
	defer func() {
		for _, e := range os.Environ() {
			pair := strings.Split(e, "=")
			if strings.HasPrefix(pair[0], "DUFFLE_ACI_DRIVER") {
				t.Logf("Unsetting Env Variable: %s", pair[0])
				os.Unsetenv(pair[0])
			}
		}
	}()

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
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
	}
	d, err := NewACIDriver()
	assert.NoErrorf(t, err, "Expected no error when creating Driver to run operation. Got: %v", err)
	assert.NotNil(t, d)
	err = d.Run(&op)
	assert.NoErrorf(t, err, "Expected no error when running Test Operation. Got: %v", err)
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
