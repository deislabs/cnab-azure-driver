package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/spf13/cobra"

	"github.com/deislabs/duffle-aci-driver/pkg/driver"
)

// Version is the current version of duffle-aci-driver
var Version string

var handles bool

var rootCmd = &cobra.Command{
	Use:          "duffle-aci-driver",
	Short:        "duffle-aci-driver is a duffle driver to execute CNAB actions",
	Long:         `A duffle driver to execute CNAB actions using Azure ACI`,
	RunE:         runRootCmd,
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the aci driver version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("duffle-aci-driver version:%v \n", Version)
	},
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

	acidriver, err := driver.NewACIDriver()
	if err != nil {
		return fmt.Errorf("Error creating ACI Driver: %v", err)
	}

	fmt.Printf("Running %s action on %s\n", op.Action, op.Installation)
	return acidriver.Run(op)

}

// HandlesImageTypes writes output containing comma separated values list of imageTypes that the ACI Driver can handle
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

	if fi.Size() == 0 && (fi.Mode()&os.ModeNamedPipe == 0) {
		return nil, errors.New("No input passed on stdin")
	}

	bytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("Error reading from stdin: %v", err)
	}

	fmt.Println(string(bytes))
	if err = json.Unmarshal(bytes, &op); err != nil {
		return nil, fmt.Errorf("Error getting bundle.json: %v", err)
	}

	return &op, nil
}

func init() {
	rootCmd.Flags().BoolVarP(&handles, "handles", "", false, "Checks if driver supports Invocation Image type being executed")
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the aci command driver
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
