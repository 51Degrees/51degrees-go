package common

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Returns a full path to a file to be used for examples by path to a file
func GetFilePathByPath(path string) string {
	dir, file := filepath.Split(path)
	filePath, err := GetFilePath(
		dir,
		[]string{file},
	)
	if err != nil {
		log.Fatalf("Could not find any file that matches \"%s\" at path \"%s\".\n",
			file,
			dir)
	}
	return filePath
}

// GetFilePath take a directory and search for the target file in that directory,
// including the subdirectories. The directory is a relative path to the current
// directory of the caller file.
func GetFilePath(dir string, names []string) (found string, err error) {
	// Target directory
	var path string
	if filepath.IsAbs(dir) {
		path = dir
	} else {
		rootDir, e := os.Getwd()
		if e != nil {
			log.Fatalln("Failed to get current directory.")
		}
		path = filepath.Join(rootDir, dir)
	}

	// walk through the directory
	foundPath := ""
	e := filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error \"%s\" occured on path \"%s\".\n",
				err,
				path)
			return err
		}
		for _, name := range names {
			if strings.EqualFold(info.Name(), name) {
				foundPath = path
				// Return error Exist here to stop the walk
				return fs.ErrExist
			}
		}
		return nil
	})

	if e != fs.ErrExist {
		return foundPath, fs.ErrNotExist
	}

	return foundPath, nil
}
