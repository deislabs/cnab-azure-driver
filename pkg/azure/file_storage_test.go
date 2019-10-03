package azure

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/deislabs/cnab-azure-driver/test"
)

var runAzureTest = flag.Bool("runazuretest", false, "Run tests in Azure")
var verboseDriver = flag.Bool("verbosedriveroutput", false, "Set Verbose Output in Azure Driver")

func TestAzureFileShare(t *testing.T) {
	testcases := []struct {
		name    string
		key     string
		message string
	}{
		{"", "", "Expected Error when account name and key are not set"},
		{"badname", "badkey", "Expected Error when account name and key are invalid"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewFileShare(tc.name, tc.key, "")
			assert.Error(t, err, tc.message)
		})
	}
}
func TestAzureFileShareInAzure(t *testing.T) {
	testShareDetails := setUpAzureTest(t)
	defer test.UnSetDriverEnvironmentVars(t)
	testcases := []struct {
		name        string
		accountName string
		accountKey  string
		shareName   string
		expectError bool
		message     string
	}{
		{"Valid Storage and Share", "accountName", "accountKey", "shareName", false, "Expected No Error when creating AzureFileShare"},
		{"No Share", "accountName", "accountKey", "", true, "Expected Error when creating AzureFileShare with no share"},
		{"Share does not exist", "accountName", "accountKey", "dontexist", true, "Expected Error when creating AzureFileShare with non-existent share"},
	}
	testValues := map[string]string{
		"shareName":   "",
		"accountName": "",
		"accountKey":  "",
	}
	for _, tc := range testcases {
		for k := range testValues {
			value := test.GetFieldValue(t, tc, k).(string)
			if _, exists := testValues[value]; exists {
				testValues[value] = testShareDetails[value]
			} else {
				testValues[k] = value
			}

		}
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewFileShare(testValues["accountName"], testValues["accountKey"], testValues["shareName"])
			if tc.expectError {
				assert.Error(t, err, tc.message)
			} else {
				assert.NoError(t, err, tc.message)
			}
		})
	}
}
func TestReadAndWriteFilesToShare(t *testing.T) {
	testShareDetails := setUpAzureTest(t)
	testcases := []struct {
		name          string
		fileName      string
		overwrite     bool
		expectError   bool
		read          bool
		contentLength int
		message       string
	}{
		{"File does not exist 1", "testfile1", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 2", "/testfile2/", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 3", "testfile3/", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 4", "/testfile4", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 5", "testdir/testfile5", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 6", "/testdir/testfile6", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 7", "/testdir/testfile7/", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 8", "testdir/testfile8/", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 9", "testdir/testdir/testfile9", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 10", "/testdir/testdir/testfile10", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 11", "/testdir/testdir/testfile11/", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File does not exist 12", "testdir/testdir/testfile12/", false, false, false, 1024, "Expected No Error when creating new file in Azure Share"},
		{"File already exists and not overwritten 1", "testfile1", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 2", "/testfile2/", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 3", "testfile3/", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 4", "/testfile4", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 5", "testdir/testfile5", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 6", "/testdir/testfile6/", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 7", "testdir/testfile7/", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 8", "/testdir/testfile8", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 9", "testdir/testdir/testfile9", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 10", "/testdir/testdir/testfile10/", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 11", "testdir/testdir/testfile11/", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 12", "/testdir/testdir/testfile12", false, true, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists 1", "testfile1", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 2", "/testfile2/", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 3", "testfile3/", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 4", "/testfile4", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 5", "testdir/testfile5", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 6", "/testdir/testfile6/", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 7", "testdir/testfile7/", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 8", "/testdir/testfile8", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 9", "/testdir/testdir/testfile9", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 10", "/testdir/testdir/testfile10/", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 11", "testdir/testdir/testfile11/", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 12", "/testdir/testdir/testfile12", true, false, true, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File is too big", "testfile", true, true, false, (1 << 22) + 1, "Expected Error when trying to create file larger than 4MB"},
	}
	directory := uuid.New().String()
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fileName := fmt.Sprintf("%s/%s", directory, tc.fileName)
			afs, err := NewFileShare(testShareDetails["accountName"], testShareDetails["accountKey"], testShareDetails["shareName"])
			assert.NoError(t, err, "Expected no error creating AzureFileShare")
			if tc.read {
				_, err = afs.ReadFileFromShare(fileName)
			}
			assert.NoError(t, err, tc.message)
			content := createContent(tc.contentLength)
			err = afs.WriteFileToShare(fileName, content, tc.overwrite)
			if tc.expectError {
				assert.Error(t, err, tc.message)
			} else {
				assert.NoError(t, err, tc.message)
				contentRead, err := afs.ReadFileFromShare(fileName)
				assert.NoError(t, err, "Error reading content from file %s in fileshare %s", tc.fileName, afs.share.Name)
				contentWritten := string(content)
				assert.Equal(t, contentWritten, contentRead, "Content from file %s in fileshare %s not equal to expected content", tc.fileName, afs.share.Name)
			}
		})
	}
	defer test.UnSetDriverEnvironmentVars(t)
}
func createContent(size int) []byte {
	b := make([]byte, 0, size)
	for i := 0; i < size; i++ {
		b = append(b, 0xff)
	}
	return b
}
func setUpAzureTest(t *testing.T) map[string]string {
	if !*runAzureTest {
		t.Skip("Not running tests in Azure")
	}

	test.SetLoggingLevel(verboseDriver)
	test.UnSetDriverEnvironmentVars(t)
	testShareDetails := map[string]string{
		"shareName":   "CNAB_AZURE_STATE_FILESHARE",
		"accountName": "CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME",
		"accountKey":  "CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY",
	}
	for k, v := range testShareDetails {
		envvarName := fmt.Sprintf("TEST_%s", v)
		envvar := os.Getenv(envvarName)
		if len(envvar) == 0 {
			t.Logf("Expected Test Environment Variable %s not Set", envvarName)
			t.FailNow()
		}
		t.Logf("Setting Test Value %s=%s", k, envvar)
		testShareDetails[k] = envvar
	}
	return testShareDetails
}
