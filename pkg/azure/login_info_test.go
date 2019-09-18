package azure

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLoginInfo(t *testing.T) {

	testcases := []struct {
		name         string
		envVarsToGet []string
		valueToCheck LoginType
	}{
		// Set environments vars using TEST_
		{"Get LoginInfo using Service Principal", []string{"DUFFLE_ACI_DRIVER_CLIENT_ID", "DUFFLE_ACI_DRIVER_CLIENT_SECRET", "DUFFLE_ACI_DRIVER_TENANT_ID"}, ServicePrincipal},
		{"Get LoginInfo using Service Principal", []string{"DUFFLE_ACI_DRIVER_APP_ID"}, DeviceCode},
	}

	for _, tc := range testcases {

		t.Run(tc.name, func(t *testing.T) {
			var config = map[string]string{
				"DUFFLE_ACI_DRIVER_CLIENT_ID":     "",
				"DUFFLE_ACI_DRIVER_CLIENT_SECRET": "",
				"DUFFLE_ACI_DRIVER_TENANT_ID":     "",
				"DUFFLE_ACI_DRIVER_APP_ID":        "",
			}
			params := 0
			// Check for environment variables to use for login these are expected to be the name of the relevant driver variable prefixed with TEST_
			for _, e := range tc.envVarsToGet {
				envvar := os.Getenv(fmt.Sprintf("TEST_%s", e))
				if len(envvar) > 0 {
					config[e] = envvar
					params++
				}
			}
			if params == len(tc.envVarsToGet) {
				t.Log("Testing Login in with Service Principal")
				loginInfo, err := LoginToAzure(config["DUFFLE_ACI_DRIVER_CLIENT_ID"], config["DUFFLE_ACI_DRIVER_CLIENT_SECRET"], config["DUFFLE_ACI_DRIVER_TENANT_ID"], config["DUFFLE_ACI_DRIVER_APP_ID"])
				assert.NoError(t, err)
				assert.Equal(t, ServicePrincipal, loginInfo.LoginType, "Expected Login type to be %v", ServicePrincipal)
			} else {
				t.Skip("Skipping Test Not All Environment Variables Set")
			}
		})

	}
}
