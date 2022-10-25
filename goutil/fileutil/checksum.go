package fileutil

import (
	"crypto/sha1"
	"encoding/hex"
	"hash"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// DirSha1 computes sha1 of a directory. It walks all files in lex order
// and computes the hash of all data
func DirSha1(dirPath string, seed string) (string, error) {
	hasher := sha1.New()
	if _, err := hasher.Write([]byte(seed)); err != nil {
		return "", errors.Wrap(err, "error writing seed to hasher")
	}

	walkDirFunc := getWalkDirFunc(hasher, dirPath)
	err := filepath.WalkDir(dirPath, walkDirFunc)

	if err != nil {
		return "",
			errors.Wrap(err, "error computing hash of directory")
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func getWalkDirFunc(hasher hash.Hash, rootPath string) func(path string, d fs.DirEntry, err error) error {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.WithStack(err)
		}
		var data []byte
		if d.IsDir() {
			// use the relative path so that if the `dirPath` is within
			// a temp-dir, then a different temp-dir but same
			// contents results in the same hash string
			relPath, err := filepath.Rel(rootPath, path)
			if err != nil {
				return errors.Wrapf(
					err,
					"failed to get relative path from path (%s) and rootPath (%s)",
					path,
					rootPath,
				)
			}
			data = []byte(relPath)
		} else {
			info, err := d.Info()
			if err != nil {
				return errors.WithStack(err)
			}

			// If file is a symlink, walk through the content of the symlink instead.
			if info.Mode().Type() == fs.ModeSymlink {
				symLinkPath, err := filepath.EvalSymlinks(path)
				if err != nil {
					return errors.WithStack(err)
				}
				return filepath.WalkDir(symLinkPath, getWalkDirFunc(hasher, rootPath))
			}

			data, err = os.ReadFile(path)
			if err != nil {
				return errors.Wrapf(
					err,
					"error reading file at path %s",
					path,
				)
			}
		}
		_, err = hasher.Write(data)
		if err != nil {
			return errors.Wrap(err, "error writing to hasher")
		}
		return nil
	}
}
