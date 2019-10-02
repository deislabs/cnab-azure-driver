package azure

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/storage"
)

type FileShare struct {
	share *storage.Share
}

func (afs *FileShare) ReadFileFromShare(fileName string) (string, error) {
	log.Debugf("Reading %s from share %s", fileName, afs.share.Name)
	fileName = getCleanFileName(fileName)
	file := afs.share.GetRootDirectoryReference().GetFileReference(fileName)
	if exists, err := file.Exists(); err != nil || !exists {
		if err != nil {
			return "", fmt.Errorf("Error checking if file %s exists in FileShare %s: %v", fileName, afs.share.Name, err)
		}
		return "", fmt.Errorf("File %s not found in FileShare %s", fileName, afs.share.Name)
	}
	options := storage.FileRequestOptions{}
	stream, err := file.DownloadToStream(&options)
	if err != nil {
		return "", fmt.Errorf("Error downloading from file %s in fileshare %s Error: %v", fileName, afs.share.Name, err)
	}

	defer stream.Close()
	content, err := ioutil.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("Error reading from file %s in fileshare %s Error: %v", fileName, afs.share.Name, err)
	}

	return string(content), nil
}
func (afs *FileShare) WriteFileToShare(fileName string, content []byte, overwrite bool) error {
	cleanFileName, cleanDirName := getCleanFileNameParts(fileName)
	var cleanPath string
	if len(cleanDirName) > 0 {
		log.Debugf("Clean Dir Name:%s", cleanDirName)
		// cannot use filepath.Split() or os.PathSeperator as this will fail if this code is run on Windows
		for _, name := range strings.Split(cleanDirName, "/") {
			cleanPath = path.Join(cleanPath, name)
			log.Debugf("Creating Directory:%s", cleanPath)
			dir := afs.share.GetRootDirectoryReference().GetDirectoryReference(cleanPath)
			if _, err := dir.CreateIfNotExists(nil); err != nil {
				return fmt.Errorf("Error creating directory %s in FileShare %s: %v", cleanPath, afs.share.Name, err)
			}
		}
	}

	cleanFileName = path.Join(cleanPath, cleanFileName)
	log.Debugf("Clean File Name:%s", cleanFileName)
	file := afs.share.GetRootDirectoryReference().GetFileReference(cleanFileName)
	if exists, err := file.Exists(); err != nil || (exists && !overwrite) {
		if err != nil {
			return fmt.Errorf("Error checking if file %s exists in FileShare %s: %v", fileName, afs.share.Name, err)
		}
		return fmt.Errorf("File %s already exists in FileShare %s", fileName, afs.share.Name)
	}

	size := uint64(len(content))
	err := file.Create(uint64(len(content)), nil)
	if err != nil {
		return fmt.Errorf("Error creating file %s in FileShare %s Error:%v", fileName, afs.share.Name, err)
	}

	writeRangeOptions := storage.WriteRangeOptions{
		ContentMD5: getMD5HashAsBase64(content),
	}
	err = file.WriteRange(bytes.NewReader(content), storage.FileRange{Start: 0, End: size - 1}, &writeRangeOptions)
	if err != nil {
		return fmt.Errorf("Error writing file %s in FileShare %s Error:%v", fileName, afs.share.Name, err)
	}

	return nil
}
func getMD5HashAsBase64(content []byte) string {
	hash := md5.Sum(content)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// NewFileShare creates a new AzureFileShare client
func NewFileShare(accountName string, accountKey string, shareName string) (*FileShare, error) {
	afs := FileShare{
		share: nil,
	}
	baseclient, err := storage.NewBasicClient(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("Error getting Storage Client when creating FileShareClient: %v", err)
	}

	client := baseclient.GetFileService()
	afs.share = client.GetShareReference(shareName)
	if exists, err := afs.share.Exists(); err != nil || !exists {
		if err != nil {
			return nil, fmt.Errorf("Error checking if share %s exists in Storage Account %s: %v", shareName, accountName, err)
		}
		return nil, fmt.Errorf("Azure Share %s does not exist in Storage Account %s", shareName, accountName)
	}

	return &afs, nil
}
func getCleanFileNameParts(fileName string) (cleanFileName string, cleanDirName string) {
	dirPath, fileNameOnly := path.Split(strings.TrimSuffix(fileName, "/"))
	cleanFileName = path.Clean(fileNameOnly)
	cleanDirName = path.Clean(dirPath)
	return
}
func getCleanFileName(fileName string) (cleanFileName string) {
	return path.Clean(strings.TrimSuffix(fileName, "/"))
}
