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
func TestFilesShareFileHandling(t *testing.T) {
	testShareDetails := setUpAzureTest(t)
	testcases := []struct {
		name          string
		fileName      string
		checkexists   bool
		fileexists    bool
		read          bool
		write         bool
		delete        bool
		overwrite     bool
		expectError   bool
		contentLength int
		message       string
	}{
		{"No Filename 1", "/testfile/", true, false, false, false, false, false, true, 0, "Expected Error when checking for a file in Azure Share with no filename"},
		{"No Filename 2", "testfile/", true, false, false, false, false, false, true, 0, "Expected Error when checking for a file in Azure Share with no filename"},
		{"No Filename 3", "/testdir/testfile/", true, false, false, false, false, false, true, 0, "Expected Error when checking for a file in Azure Share with no filename"},
		{"No Filename 4", "testdir/testfile/", true, false, false, false, false, false, true, 0, "Expected Error when checking for a file in Azure Share with no filename"},
		{"No Filename 5", "/testdir/testdir/testfile/", true, false, false, false, false, false, true, 0, "Expected Error when checking for a file in Azure Share with no filename"},
		{"No Filename 6", "testdir/testdir/testfile/", true, false, false, false, false, false, true, 0, "Expected Error when checking for a file in Azure Share with no filename"},
		{"Read No Filename 1", "/testfile/", false, true, false, false, false, false, true, 0, "Expected Error when reading a file in Azure Share with no filename"},
		{"Read No Filename 2", "testfile/", false, true, false, false, false, false, true, 0, "Expected Error when reading a file in Azure Share with no filename"},
		{"Read No Filename 3", "/testdir/testfile/", false, true, false, false, false, false, true, 0, "Expected Error when reading a file in Azure Share with no filename"},
		{"Read No Filename 4", "testdir/testfile/", false, true, false, false, false, false, true, 0, "Expected Error when reading a file in Azure Share with no filename"},
		{"Read No Filename 5", "/testdir/testdir/testfile/", false, true, false, false, false, false, true, 0, "Expected Error when reading a file in Azure Share with no filename"},
		{"Read No Filename 6", "testdir/testdir/testfile/", false, true, false, false, false, false, true, 0, "Expected Error when reading a file in Azure Share with no filename"},
		{"Write No Filename 1", "/testfile/", false, false, true, false, false, false, true, 1024, "Expected Error when writing a file in Azure Share with no filename"},
		{"Write No Filename 2", "testfile/", false, false, true, false, false, false, true, 1024, "Expected Error when writing a file in Azure Share with no filename"},
		{"Write No Filename 3", "/testdir/testfile/", false, false, true, false, false, false, true, 1024, "Expected Error when writing a file in Azure Share with no filename"},
		{"Write No Filename 4", "testdir/testfile/", false, false, true, false, false, false, true, 1024, "Expected Error when writing a file in Azure Share with no filename"},
		{"Write No Filename 5", "/testdir/testdir/testfile/", false, false, true, false, false, false, true, 1024, "Expected Error when writing a file in Azure Share with no filename"},
		{"Write No Filename 6", "testdir/testdir/testfile/", false, false, true, false, false, false, true, 1024, "Expected Error when writing a file in Azure Share with no filename"},
		{"Check File does not exist 1", "testfile1", true, false, false, false, false, false, false, 0, "Expected No Error when checking file exists in Azure Share"},
		{"Check File does not exist 2", "/testfile2", true, false, false, false, false, false, false, 0, "Expected No Error when checking file exists in Azure Share"},
		{"Check File does not exist 3", "testdir/testfile3", true, false, false, false, false, false, false, 0, "Expected No Error when checking file exists in Azure Share"},
		{"Check File does not exist 4", "/testdir/testfile4", true, false, false, false, false, false, false, 0, "Expected No Error when checking file exists in Azure Share"},
		{"Check File does not exist 5", "testdir/testdir/testfile5", true, false, false, false, false, false, false, 0, "Expected No Error when checking file exists in Azure Share"},
		{"Check File does not exist 6", "/testdir/testdir/testfile6", true, false, false, false, false, false, false, 0, "Expected No Error when checking file exists in Azure Share"},
		{"Read File does not exist 1", "testfile1", true, false, false, false, false, false, false, 0, "Expected No Error when reading file that does not exist in Azure Share"},
		{"Read File does not exist 2", "/testfile2", true, false, false, false, false, false, false, 0, "Expected No Error when reading file that does not exist in Azure Share"},
		{"Read File does not exist 3", "testdir/testfile3", true, false, false, false, false, false, false, 0, "Expected No Error when reading file that does not exist in Azure Share"},
		{"Read File does not exist 4", "/testdir/testfile4", true, false, false, false, false, false, false, 0, "Expected No Error when reading file that does not exist in Azure Share"},
		{"Read File does not exist 5", "testdir/testdir/testfile5", true, false, false, false, false, false, false, 0, "Expected No Error when reading file that does not exist in Azure Share"},
		{"Read File does not exist 6", "/testdir/testdir/testfile6", true, false, false, false, false, false, false, 0, "Expected No Error when reading file that does not exist in Azure Share"},
		{"Write File does not exist 1", "testfile1", true, false, false, true, false, false, false, 1024, "Expected No Error when writing file that does not exist in Azure Share"},
		{"Write File does not exist 2", "/testfile2", true, false, false, true, false, false, false, 1024, "Expected No Error when writing file that does not exist in Azure Share"},
		{"Write File does not exist 3", "testdir/testfile3", true, false, false, true, false, false, false, 1024, "Expected No Error when writing file that does not exist in Azure Share"},
		{"Write File does not exist 4", "/testdir/testfile4", true, false, false, true, false, false, false, 1024, "Expected No Error when writing file that does not exist in Azure Share"},
		{"Write File does not exist 5", "testdir/testdir/testfile5", true, false, false, true, false, false, false, 1024, "Expected No Error when writing file that does not exist in Azure Share"},
		{"Write File does not exist 6", "/testdir/testdir/testfile6", true, false, false, true, false, false, false, 1024, "Expected No Error when writing file that does not exist in Azure Share"},
		{"File already exists and not overwritten 1", "testfile1", true, true, true, true, false, false, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 2", "/testfile2", true, true, true, true, false, false, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 3", "testdir/testfile3", true, true, true, true, false, false, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 4", "/testdir/testfile4", true, true, true, true, false, false, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 5", "testdir/testdir/testfile5", true, true, true, true, false, false, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists and not overwritten 6", "/testdir/testdir/testfile6", true, true, true, true, false, false, true, 1024, "Expected Error when trying to create a new file that already exists and not overwriting existing file"},
		{"File already exists 1", "testfile1", true, true, true, true, true, true, false, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 2", "/testfile2", true, true, true, true, true, true, false, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 3", "testdir/testfile3", true, true, true, true, true, true, false, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 4", "/testdir/testfile4", true, true, true, true, true, true, false, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 5", "/testdir/testdir/testfile5", true, true, true, true, true, true, false, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File already exists 6", "/testdir/testdir/testfile6", true, true, true, true, true, true, false, 2048, "Expected No Error when overwriting existing file in Azure Share"},
		{"File is too big", "largefile", true, false, false, true, false, false, true, (1 << 22) + 1, "Expected Error when trying to create file larger than 4MB"},
	}
	directory := uuid.New().String()
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fileName := fmt.Sprintf("%s/%s", directory, tc.fileName)
			afs, err := NewFileShare(testShareDetails["accountName"], testShareDetails["accountKey"], testShareDetails["shareName"])
			assert.NoError(t, err, "Expected no error creating AzureFileShare")
			if tc.checkexists {
				exists, err := afs.CheckIfFileExists(fileName)
				if !tc.read && !tc.write && tc.expectError {
					assert.Error(t, err, tc.message)
				} else {
					assert.NoError(t, err, "Expected no error Checking if file exists")
					assert.Equal(t, tc.fileexists, exists, "Expected File existence: %v got: %v", tc.fileexists, exists)
				}
			}
			if tc.read {
				_, err = afs.ReadFileFromShare(fileName)
				if !tc.write && !tc.delete && tc.expectError {
					assert.Error(t, err, tc.message)
				}
			}
			if tc.write {
				content := createContent(tc.contentLength)
				err = afs.WriteFileToShare(fileName, content, tc.overwrite)
				if !tc.delete && tc.expectError {
					assert.Error(t, err, tc.message)
				} else {
					assert.NoError(t, err, tc.message)
					contentRead, err := afs.ReadFileFromShare(fileName)
					assert.NoError(t, err, "Error reading content from file %s in fileshare %s", tc.fileName, afs.share.Name)
					contentWritten := string(content)
					assert.Equal(t, contentWritten, contentRead, "Content from file %s in fileshare %s not equal to expected content", tc.fileName, afs.share.Name)
				}
			}
			if tc.delete {
				deleted, err := afs.DeleteFileFromShare(fileName)
				assert.NoError(t, err, tc.message)
				assert.True(t, deleted)
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
		testShareDetails[k] = envvar
	}
	return testShareDetails
}
