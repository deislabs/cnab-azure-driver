package azure

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLoginInfo(t *testing.T) {

	testcases := []struct {
		name         string
		runTest      bool
		envVarsToGet []string
		valueToCheck LoginType
	}{
		// to execute tests set environment vars using names in envVarsToGet preceded with TEST_
		// TODO add tests for other login methods
		{"Get LoginInfo using Service Principal", true, []string{"CNAB_AZURE_CLIENT_ID", "CNAB_AZURE_CLIENT_SECRET", "CNAB_AZURE_TENANT_ID"}, ServicePrincipal},
		{"Get LoginInfo using Azure CLI", shouldRunTest("CNAB_AZURE_RUN_CLI_LOGIN_TEST"), []string{}, CLI},
	}

	for _, tc := range testcases {
		if tc.runTest {
			t.Run(tc.name, func(t *testing.T) {
				var config = map[string]string{
					"CNAB_AZURE_CLIENT_ID":     "",
					"CNAB_AZURE_CLIENT_SECRET": "",
					"CNAB_AZURE_TENANT_ID":     "",
					"CNAB_AZURE_APP_ID":        "",
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
					t.Log("Testing", tc.name)
					loginInfo, err := LoginToAzure(config["CNAB_AZURE_CLIENT_ID"], config["CNAB_AZURE_CLIENT_SECRET"], config["CNAB_AZURE_TENANT_ID"], config["CNAB_AZURE_APP_ID"])
					assert.Equal(t, tc.valueToCheck, loginInfo.LoginType, "Expected Login type to be %v", tc.valueToCheck)
					assert.NoError(t, err)
				} else {
					t.Skip(fmt.Sprintf("Skipping Test %s Not All Required Environment Variables Set", tc.name))
				}

			})
		} else {
			t.Skip(fmt.Sprintf("Skipping Test %s as runTest is false", tc.name))
		}
	}
}

func shouldRunTest(envVarName string) bool {
	envvar := os.Getenv(fmt.Sprintf("TEST_%s", envVarName))
	if len(envvar) > 0 {
		result, _ := strconv.ParseBool(envvar)
		return result
	}
	return false
}
