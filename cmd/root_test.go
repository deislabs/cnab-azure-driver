package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/deislabs/cnab-go/bundle"
	cnabdriver "github.com/deislabs/cnab-go/driver"
	"github.com/stretchr/testify/assert"
)

func TestHandlesImageTypes(t *testing.T) {
	actual := getOutput(t, HandlesImageTypes)
	expected := fmt.Sprintf("%s,%s\n", cnabdriver.ImageTypeDocker, cnabdriver.ImageTypeOCI)
	assert.Equalf(t, expected, actual, "Handles output error - Expected: %s Got: %s", expected, actual)
}

func TestInput(t *testing.T) {
	_, err := GetOperation()
	assert.Errorf(t, err, "Expected Error when no input provided on stdin")
	_, err = writeToStdInAndTest([]byte("invalid input"), GetOperation)
	assert.Errorf(t, err, "Expected Error when invalid input provided on stdin")
	bytes, err := ioutil.ReadFile("testdata/operation-test.json")
	assert.NoError(t, err, "Error reading writing to stdin")
	result, err := writeToStdInAndTest(bytes, GetOperation)
	var op *cnabdriver.Operation
	if result == nil {
		t.Error("Got nil value returned from GetOperation")
		t.Fail()
	}
	val, ok := result.(reflect.Value)
	if !ok || (val.Interface() == nil) {
		t.Error("Got nil value or invalid type returned from GetOperation")
		t.Fail()
	} else {

		op = val.Interface().(*cnabdriver.Operation)
	}
	assert.NoError(t, err, "Testing user input")
	expectedOp := cnabdriver.Operation{
		Action:       "install",
		Installation: "test",
		Parameters: map[string]interface{}{
			"param1": "value1",
			"param2": "value2",
		},
		Image: bundle.InvocationImage{
			BaseImage: bundle.BaseImage{
				Image:     "testing.azurecr.io/duffle/test",
				ImageType: "docker",
				Digest:    "sha256:ba27c336615454378b0c1d85ef048583b1fd607b1a96defc90988292e9fb1edb"},
		},
		Revision: "01DDY0MT808KX0GGZ6SMXN4TW",
		Environment: map[string]string{
			"ENV_1": "value1",
			"ENV_2": "value2",
		},
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
	}
	assert.True(t, reflect.DeepEqual(expectedOp, *op), "Validating value of operation from stdin failed")
}
func TestOutputHandling(t *testing.T) {
	bytes, err := ioutil.ReadFile("testdata/output-test.json")
	assert.NoError(t, err, "Error reading from testdata/output-test.json")
	_, err = writeToStdInAndTest(bytes, RunOperation)
	assert.EqualError(t, err, "Bundle has 2 outputs but CNAB_OUTPUT_DIR is not set")
	os.Setenv("CNAB_OUTPUT_DIR", "")
	_, err = writeToStdInAndTest(bytes, RunOperation)
	assert.EqualError(t, err, "Bundle has 2 outputs but CNAB_OUTPUT_DIR is not set")
	tempDirName, err := ioutil.TempDir("", "outputtest")
	if err != nil {
		t.Error("Failed to create temp dir 1")
		t.Fail()
	}
	os.Remove(tempDirName)
	os.Setenv("CNAB_OUTPUT_DIR", tempDirName)
	_, err = writeToStdInAndTest(bytes, RunOperation)
	errmsg := fmt.Sprintf("CNAB_OUTPUT_DIR: %s does not exist", tempDirName)
	assert.EqualError(t, err, errmsg)
}

func getOutput(t *testing.T, f func()) string {
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()
	r, w, err := os.Pipe()
	assert.Nilf(t, err, "os.Pipe call failed: %v", err)
	os.Stdout = w
	f()
	err = w.Close()
	assert.Nilf(t, err, "Closing stdout Writer failed: %v", err)
	output, err := ioutil.ReadAll(r)
	assert.Nilf(t, err, "Reading stdout failed: %v", err)
	return string(output)
}

func writeToStdInAndTest(content []byte, fn interface{}) (interface{}, error) {
	tempstdin, err := ioutil.TempFile("", "testinput")
	if err != nil {
		return nil, fmt.Errorf("Error creating temp file for stdin: %v", err)
	}

	defer os.Remove(tempstdin.Name())

	if _, err := tempstdin.Write(content); err != nil {
		return nil, fmt.Errorf("Error writing content to temp file for stdin: %v", err)
	}

	defer tempstdin.Close()

	if _, err := tempstdin.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("Error rewinding temp file for stdin: %v", err)
	}

	stdin := os.Stdin
	defer func() { os.Stdin = stdin }()
	os.Stdin = tempstdin
	if reflect.TypeOf(fn) == nil {
		return nil, errors.New("Test function is nil")
	}

	v := reflect.ValueOf(fn)
	switch reflect.TypeOf(fn) {
	case reflect.TypeOf(GetOperation):
		args := []reflect.Value{}
		result := v.Call(args)
		var err error
		if result[1].Interface() == nil {
			err = nil
		} else {
			err = result[1].Interface().(error)
		}
		return result[0], err
	case reflect.TypeOf(RunOperation):
		args := []reflect.Value{}
		result := v.Call(args)
		var err error
		if result[0].Interface() == nil {
			err = nil
		} else {
			err = result[0].Interface().(error)
		}
		return nil, err
	default:
		return nil, errors.New("Unknown function type to test")
	}
}
