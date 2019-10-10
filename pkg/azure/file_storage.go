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
	if exists, err := afs.CheckIfFileExists(fileName); err != nil || !exists {
		if err != nil {
			return "", fmt.Errorf("Error checking if file %s exists in FileShare %s: %v", fileName, afs.share.Name, err)
		}
		return "", fmt.Errorf("File %s not found in FileShare %s", fileName, afs.share.Name)
	}
	file := afs.share.GetRootDirectoryReference().GetFileReference(path.Clean(fileName))
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
	log.Debugf("FileName:%s CleanFileName:%s CleanDirName:%s", fileName, cleanFileName, cleanDirName)
	if len(cleanFileName) == 0 {
		return fmt.Errorf("No Filename in path: %s", fileName)
	}
	if _, err := afs.checkIfDirExistsAndCreate(cleanDirName, true); err != nil {
		return fmt.Errorf("Error checking if file %s exists in FileShare %s: %v", fileName, afs.share.Name, err)
	}

	cleanFileName = path.Join(cleanDirName, cleanFileName)
	log.Debugf("Full Clean File Name: %s", cleanFileName)
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
	dirPath, cleanFileName := path.Split(fileName)
	// Root Directory returns "/"
	cleanDirName = strings.Trim(path.Clean(dirPath), "/")
	return
}
func (afs *FileShare) checkIfDirExists(dirPath string) (bool, error) {
	return afs.checkIfDirExistsAndCreate(dirPath, false)
}
func (afs *FileShare) checkIfDirExistsAndCreate(dirPath string, create bool) (bool, error) {
	log.Debugf("Checking if dir %s exists in share %s", dirPath, afs.share.Name)
	dirPath = strings.Trim(path.Clean(dirPath), "/")
	log.Debugf("Clean Dir Path: %s", dirPath)
	cleanPath := ""
	if len(dirPath) > 0 {
		// cannot use filepath.Split() or os.PathSeperator as this will fail if this code is run on Windows
		for _, name := range strings.Split(dirPath, "/") {
			cleanPath = path.Join(cleanPath, name)
			log.Debugf("Checking if dirPath exists: %s", cleanPath)
			dir := afs.share.GetRootDirectoryReference().GetDirectoryReference(cleanPath)
			if exists, err := dir.Exists(); err != nil || !exists {
				if err != nil {
					return false, fmt.Errorf("Error checking if directory %s exists in share %s: %v", cleanPath, afs.share.Name, err)
				}
				if create {
					log.Debugf("Creating directory: %s", cleanPath)
					if _, err := dir.CreateIfNotExists(nil); err != nil {
						return false, fmt.Errorf("Error creating directory %s in FileShare %s: %v", cleanPath, afs.share.Name, err)
					}
					log.Debugf("dirPath created: %s", cleanPath)
				} else {
					log.Debugf("dirPath does not exist: %s", cleanPath)
					return false, nil
				}
			}
			log.Debugf("dirPath exists: %s", cleanPath)
		}
	}
	return true, nil

}
func (afs *FileShare) CheckIfFileExists(fileName string) (bool, error) {
	log.Debugf("Checking if %s exists in share %s", fileName, afs.share.Name)
	cleanFileName, cleanDirName := getCleanFileNameParts(fileName)
	if len(cleanFileName) == 0 {
		return false, fmt.Errorf("No Filename in path: %s", fileName)
	}
	log.Debugf("FileName:%s CleanFileName:%s CleanDirName:%s", fileName, cleanFileName, cleanDirName)
	dir := afs.share.GetRootDirectoryReference()
	if len(cleanDirName) > 0 {
		if exists, err := afs.checkIfDirExists(cleanDirName); err != nil || !exists {
			if err != nil {
				return false, fmt.Errorf("Error checking if file %s exists in share %s: %v", fileName, afs.share.Name, err)
			}
			return false, nil
		}
		dir = dir.GetDirectoryReference(cleanDirName)
	}
	file := dir.GetFileReference(cleanFileName)
	if exists, err := file.Exists(); err != nil || !exists {
		if err != nil {
			return false, fmt.Errorf("Error checking if file %s exists in FileShare %s: %v", fileName, afs.share.Name, err)
		}
		return false, nil
	}
	return true, nil
}
