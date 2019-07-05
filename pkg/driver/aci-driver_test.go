package driver

import (
	"os"
	"testing"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/stretchr/testify/assert"
)

func TestNewACIDriver(t *testing.T) {

	// Either DUFFLE_ACI_DRIVER_LOCATION = DUFFLE_ACI_DRIER_RESOURCE_GROUP mus be set
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

	// 	DUFFLE_ACI_DRIVER_USER_MSI_RESOURCE_ID must be set if user MSI is being used
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
}
