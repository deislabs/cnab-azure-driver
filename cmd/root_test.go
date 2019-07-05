package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

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
	_, err = testUserInput("invalid input", GetOperation)
	assert.Errorf(t, err, "Expected Error when invalid input provided on stdin")
	bytes, err := ioutil.ReadFile("testdata/operation-test.json")
	assert.NoError(t, err, "Error reading from testdata/operation-test.json")
	op, err := testUserInput(string(bytes), GetOperation)
	expectedOp := cnabdriver.Operation{
		Action:       "install",
		Installation: "test",
		Parameters: map[string]interface{}{
			"param1": "value1",
		},
		Image:     "testing.azurecr.io/duffle/test:e8966c3c153a525775cbcddd46f778bed25650b4",
		ImageType: "docker",
		Revision:  "01DDY0MT808KX0GGZ6SMXN4TW",
		Environment: map[string]string{
			"ENV_1": "value 1",
			"ENV_2": "value 2",
		},
		Files: map[string]string{
			"/cnab/app/image-map.json": "{}",
		},
	}
	assert.True(t, reflect.DeepEqual(&expectedOp, op), "Validating value of operation from stdin failed")
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

func testUserInput(content string, f func() (*cnabdriver.Operation, error)) (*cnabdriver.Operation, error) {
	tempstdin, err := ioutil.TempFile("", "testinput")
	if err != nil {
		return nil, fmt.Errorf("Error creating temp file for stdin: %v", err)
	}

	defer os.Remove(tempstdin.Name())

	if _, err := tempstdin.Write([]byte(content)); err != nil {
		return nil, fmt.Errorf("Error writing content to temp file for stdin: %v", err)
	}

	defer tempstdin.Close()

	if _, err := tempstdin.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("Error rewinding temp file for stdin: %v", err)
	}

	stdin := os.Stdin
	defer func() { os.Stdin = stdin }()

	os.Stdin = tempstdin
	return f()
}
