package main

import (
	"encoding/json"
	"fmt"

	duffleDriver "github.com/deislabs/cnab-go/driver"
<<<<<<< HEAD

	"github.com/deislabs/duffle-aci-driver/pkg"
	"github.com/deislabs/duffle-aci-driver/pkg/driver"

=======
>>>>>>> fixing output
	"github.com/spf13/cobra"

	"github.com/deislabs/duffle-aci-driver/pkg/driver"
	"io/ioutil"
	"os"
)

var handles bool
var op duffleDriver.Operation
var rootCmd = &cobra.Command{
	Use:   "duffle-aci-driver",
	Short: "duffle-aci-driver is a duffle driver to execute CNAB actions",
	Long:  `duffle-aci-driver is a duffle driver to execute CNAB actions using Azure ACI`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if handles {
			fmt.Printf("%s,%s\n", duffleDriver.ImageTypeDocker, duffleDriver.ImageTypeOCI)
			return nil
		}

		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("Error reading from stdin %v", err)
		}

		if err = json.Unmarshal(bytes, &op); err != nil {
			return fmt.Errorf("Error getting bundle.json %v", err)
		}

		acidriver := getDriver()
		if configurable, ok := acidriver.(duffleDriver.Configurable); ok {
			driverCfg := map[string]string{}
			for env := range configurable.Config() {
				driverCfg[env] = os.Getenv(env)
			}
			configurable.SetConfig(driverCfg)
		}

		fmt.Printf("Running %s action on %s\n", op.Action, op.Installation)

		return acidriver.Run(&op)

	},
	SilenceUsage: true,
}

func init() {
	rootCmd.Flags().BoolVarP(&handles, "handles", "", false, "Checks if driver supports Invocation Image type being executed")
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the application version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("duffle-aci-driver %s (%s)\n", pkg.Version, pkg.Commit)
	},
}

func getDriver() duffleDriver.Driver {
	return &driver.ACIDriver{}
}
