package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deislabs/duffle-aci-driver/pkg"
	"github.com/deislabs/duffle-aci-driver/pkg/driver"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/spf13/cobra"

	"io/ioutil"
	"os"
)

var handles bool

var rootCmd = &cobra.Command{
	Use:          "duffle-aci-driver",
	Short:        "duffle-aci-driver is a duffle driver to execute CNAB actions",
	Long:         `A duffle driver to execute CNAB actions using Azure ACI`,
	RunE:         runRootCmd,
	SilenceUsage: true,
}

func runRootCmd(cmd *cobra.Command, args []string) error {

	if handles {
		HandlesImageTypes()
		return nil
	}

	op, err := GetOperation()
	if err != nil {
		return err
	}

	acidriver := getDriver()
	if configurable, ok := acidriver.(cnabdriver.Configurable); ok {
		driverCfg := map[string]string{}
		for env := range configurable.Config() {
			driverCfg[env] = os.Getenv(env)
		}
		configurable.SetConfig(driverCfg)
	}

	fmt.Printf("Running %s action on %s\n", op.Action, op.Installation)
	return acidriver.Run(op)

}

// HandlesImageTypes writes output containing comma seperated values list of imageTypes that the ACI Driver can handle
func HandlesImageTypes() {
	fmt.Printf("%s,%s\n", cnabdriver.ImageTypeDocker, cnabdriver.ImageTypeOCI)
}

// GetOperation gets the Operation to execute
func GetOperation() (*cnabdriver.Operation, error) {
	var op cnabdriver.Operation
	fi, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("Error getting FileInfo for stdin: %v", err)
	}

	if fi.Size() == 0 {
		return nil, errors.New("No input passed on stdin")
	}

	bytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("Error reading from stdin: %v", err)
	}

	if err = json.Unmarshal(bytes, &op); err != nil {
		return nil, fmt.Errorf("Error getting bundle.json: %v", err)
	}

	return &op, nil
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
