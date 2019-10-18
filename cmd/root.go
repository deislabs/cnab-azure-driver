package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	cnabdriver "github.com/deislabs/cnab-go/driver"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/spf13/cobra"

	"github.com/deislabs/cnab-azure-driver/pkg/driver"
)

// Version is the current version of cnab-azure
var Version string
var handles bool
var verbose = false
var rootCmd = &cobra.Command{
	Use:          "cnab-azure",
	Short:        "cnab-azure is a cnab-go driver to execute CNAB actions",
	Long:         `A cnab-go driver to execute CNAB actions using Azure ACI`,
	RunE:         runRootCmd,
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the Azure driver version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cnab-azure version:%v \n", Version)
	},
}

func runRootCmd(cmd *cobra.Command, args []string) error {
	if handles {
		HandlesImageTypes()
		return nil
	}

	err := RunOperation()
	return err
}

// RunOperation a bundle operation using ACI Driver
func RunOperation() error {
	fileName, err := getLogFileName()
	if err != nil {
		return fmt.Errorf("Failed to get log filename: %v", err)
	}

	writer, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("Failed to create log filename:%s error: %v", fileName, err)
	}

	defer writer.Close()
	verboseSetting := os.Getenv("CNAB_AZURE_VERBOSE")
	if len(verboseSetting) > 0 && strings.ToLower(verboseSetting) == "true" {
		multiWriter := io.MultiWriter(os.Stdout, writer)
		log.SetOutput(multiWriter)
		verbose = true
	} else {
		log.SetOutput(writer)
	}

	log.SetLevel(log.DebugLevel)

	op, err := GetOperation()
	if err != nil {
		return logError(err)
	}

	outputDirName := os.Getenv("CNAB_OUTPUT_DIR")
	if len(op.Outputs) > 0 {
		if len(outputDirName) == 0 {
			return logError(fmt.Errorf("Bundle has %d outputs but CNAB_OUTPUT_DIR is not set", len(op.Outputs)))
		}

		// The output directory should exist and be a directory
		info, err := os.Stat(outputDirName)
		if err != nil {
			return logError(fmt.Errorf("CNAB_OUTPUT_DIR: %s does not exist", outputDirName))
		}

		if !info.IsDir() {
			return logError(fmt.Errorf("CNAB_OUTPUT_DIR: %s is not a directory", outputDirName))
		}

	}

	acidriver, err := driver.NewACIDriver(Version)
	if err != nil {
		return logError(fmt.Errorf("Error creating ACI Driver: %v", err))
	}

	fmt.Printf("Running %s action on %s\n", op.Action, op.Installation)
	opResult, err := acidriver.Run(op)
	if err != nil {
		return logError(fmt.Errorf("Running %s action on %s Error:%v", op.Action, op.Installation, err))
	}

	return logError(WriteOutputs(outputDirName, opResult))
}

// WriteOutputs writes the outputs from an operation to the location expected by the Command Driver
func WriteOutputs(outputDirName string, results cnabdriver.OperationResult) error {
	if len(results.Outputs) == 0 {
		return nil
	}

	for name, item := range results.Outputs {
		fileName := path.Clean(path.Join(outputDirName, name))
		log.Debug("Processing Output Filename ", fileName)
		dir, _ := path.Split(fileName)
		log.Debug("Creating Output Directory ", dir)
		os.MkdirAll(dir, 0744)
		err := ioutil.WriteFile(fileName, []byte(item), 0744)
		if err != nil {
			return fmt.Errorf("Failed to write output file: %s Error: %v", fileName, err)
		}
	}
	return nil
}
func logError(err error) error {
	if err != nil {
		log.Debug(err)
	}

	return err
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

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("Error reading from stdin: %v", err)
	}

	if err = json.Unmarshal(data, &op); err != nil {
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

func getLogFileName() (string, error) {

	directory := filepath.Join(os.Getenv("HOME"), ".cnab-azure-driver", "logs")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", fmt.Errorf("Error creating log directory: %s error:: %v", directory, err)
	}

	logPath := filepath.Join(directory, "cnab-azure-driver.log")
	// Just get a log filename rather than write through logFile
	logFile, err := rotatelogs.New(
		logPath+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithMaxAge(-1),
		rotatelogs.WithRotationCount(100),
		rotatelogs.ForceNewFile(),
	)
	if err != nil {
		return "", fmt.Errorf("Error getting logfile: %v", err)
	}

	// need to write to create the file name and then close the file
	if _, err := logFile.Write([]byte("")); err != nil {
		return "", fmt.Errorf("Error creating logfile: %v", err)
	}

	logFile.Close()
	return logFile.CurrentFileName(), nil
}
