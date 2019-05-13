package main

import (
	"encoding/json"
	"fmt"

<<<<<<< HEAD
	"github.com/deislabs/duffle-aci-driver/pkg"
	"github.com/deislabs/duffle-aci-driver/pkg/driver"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/spf13/cobra"

	"io/ioutil"
=======
	duffleDriver "github.com/deislabs/cnab-go/driver"

	"github.com/deislabs/duffle-aci-driver/pkg"
	"github.com/deislabs/duffle-aci-driver/pkg/driver"

	"github.com/spf13/cobra"

>>>>>>> Moved to standalone command driver
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
		} else {

			fi, err := os.Stdin.Stat()
			if err != nil {
				return fmt.Errorf("cannot stat stdin: %v", err)
			}

			if fi.Size() > 0 {
				err := json.NewDecoder(os.Stdin).Decode(&op)
				if err != nil {
					return err
				}
				acidriver := getDriver()
				if configurable, ok := acidriver.(duffleDriver.Configurable); ok {
					driverCfg := map[string]string{}
					for env := range configurable.Config() {
						driverCfg[env] = os.Getenv(env)
					}
					configurable.SetConfig(driverCfg)
				}
				return acidriver.Run(&op)
			}
			cmd.Usage()

		}
		return nil
	},
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

func getDriver() cnabdriver.Driver {
	return &driver.ACIDriver{}
}
