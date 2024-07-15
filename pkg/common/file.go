package common

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
)

// ReadFile reads the content of the file at the given path and returns it as a byte slice.
func ReadFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open file at path: %s, error: %v\n", path, err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			logrus.Errorf("Failed to close file at path: %s, error: %v", path, cerr)
		}
	}()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file at path: %s, error: %v\n", path, err)
	}

	logrus.Infof("[File] Successfully read file at path: %s", path)
	return content, nil
}

// WriteFile writes the given data to the file at the specified path, creating directories as needed.
func WriteFile(path string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directories for path: %s, error: %v\n", path, err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Failed to create file at path: %s, error: %v\n", path, err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			logrus.Errorf("Failed to close file at path: %s, error: %v", path, cerr)
		}
	}()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("Failed to write data to file at path: %s, error: %v\n", path, err)
	}

	logrus.Infof("[File] Successfully wrote data to file at path: %s", path)
	return nil
}

func DeleteFile(path string) error {
	if FileExists(path) {
		err := os.Remove(path)
		if err != nil {
			return fmt.Errorf("Failed to delete file path %s: %v\n", path, err)
		}

		logrus.Infof("[File] Successfully delete file at path: %s", path)
	}

	return nil
}

// FileExists checks if a file exists at the specified path and returns a boolean result.
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		logrus.Debugf("File exists at path: %s", filename)
		return true
	}
	if os.IsNotExist(err) {
		logrus.Debugf("File does not exist at path: %s", filename)
		return false
	}

	logrus.Errorf("[File] Failed to check file existence at path: %s, error: %v", filename, err)
	return false
}
