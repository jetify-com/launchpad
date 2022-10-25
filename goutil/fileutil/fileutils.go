package fileutil

import (
	"os"

	"github.com/pkg/errors"
)

func FileExists(path string) (bool, error) {
	fileinfo, err := os.Stat(path)
	if err == nil {
		if !fileinfo.IsDir() {
			// It is a file!
			return true, nil
		}
		// It is a directory
		return false, nil
	}

	// No such file was found:
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}

	// Some other error:
	return false, errors.WithStack(err)
}
