package common

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// The data repository, the data submodule of cxx, has a link file in
// dataFolder for each file its scripts download.
var dataFolder = filepath.Join("cxx", "data", "ip-intelligence")

// Sent with each download the examples start, so that they can be counted.
// In GitHub Actions the scripts send their own source.
const downloadSource = "examples-51degrees-go"

// Returns a full path to a file to be used for examples by path to a file. A
// file that isn't found is downloaded when the data repository links to it.
func GetFilePathByPath(path string) string {
	dir, file := filepath.Split(path)
	filePath, err := GetFilePath(
		dir,
		[]string{file},
	)
	if err == nil {
		return filePath
	}
	if filePath, found := getDataFile(file); found {
		return filePath
	}
	log.Fatalf("Could not find any file that matches \"%s\" at path \"%s\".\n",
		file,
		dir)
	return ""
}

// GetDataFile returns path when the file is there, and otherwise the data
// repository's copy of a file with the same name. It returns path when the
// data repository has no such file, so that the caller reports it missing.
func GetDataFile(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if filePath, found := getDataFile(filepath.Base(path)); found {
		return filePath
	}
	return path
}

// GetDataFolderFile returns the path of a file the data repository holds,
// such as its example evidence, looked for from the working folder up, or
// name when there is none, so that the caller reports it missing.
func GetDataFolderFile(name string) string {
	if path := findAbove(filepath.Join(dataFolder, name)); path != "" {
		return path
	}
	return name
}

// getDataFile returns the data repository's copy of the file called name,
// running the repository's script first to download it when it is missing.
// It returns false when there is no link file for name in the data folder,
// looked for from the working folder up.
func getDataFile(name string) (string, bool) {
	link := findAbove(filepath.Join(dataFolder, name+".link"))
	if link == "" {
		return "", false
	}
	path := strings.TrimSuffix(link, ".link")
	if _, err := os.Stat(path); err == nil {
		return path, true
	}

	// The data repository is the folder above the data folder.
	data := filepath.Dir(filepath.Dir(link))
	var args []string
	var command string
	if runtime.GOOS == "windows" {
		script := filepath.Join(data, "get-data-files.ps1")
		args = []string{powerShell(), "-NoProfile", "-ExecutionPolicy",
			"Bypass", "-File", script, name}
		command = fmt.Sprintf("%s -NoProfile -ExecutionPolicy Bypass "+
			"-File \"%s\" \"%s\"", args[0], script, name)
		if source := exampleSource(); source != "" {
			args = append(args, "-Source", source)
			command += " -Source " + source
		}
	} else {
		script := filepath.Join(data, "get-data-files.sh")
		args = []string{"sh", script}
		command = fmt.Sprintf("sh \"%s\"", script)
		if source := exampleSource(); source != "" {
			args = append(args, "-s", source)
			command += " -s " + source
		}
		args = append(args, name)
		command += fmt.Sprintf(" \"%s\"", name)
	}
	cmd := exec.Command(args[0], args[1:]...)
	if args[0] == "powershell" {
		cmd.Env = withoutModulePath(os.Environ())
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("%s is missing, so it is being downloaded by running\n%s\n",
		name, command)
	if cmd.Run() == nil {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	log.Fatalf("%s could not be downloaded. Run this command to try again.\n%s\n",
		name, command)
	return "", false
}

// powerShell returns the PowerShell to run the data repository's script with,
// which is PowerShell 7 when it is installed and Windows PowerShell otherwise.
func powerShell() string {
	if _, err := exec.LookPath("pwsh"); err == nil {
		return "pwsh"
	}
	return "powershell"
}

// withoutModulePath returns the environment without PSModulePath. Windows
// PowerShell started from PowerShell 7 inherits its module path, which leaves
// it unable to load its own commands, and without one it uses its own.
func withoutModulePath(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		if !strings.EqualFold(strings.SplitN(variable, "=", 2)[0], "PSModulePath") {
			kept = append(kept, variable)
		}
	}
	return kept
}

// exampleSource returns the source to send with a download, or nothing in
// GitHub Actions, where the data repository's scripts send their own so that
// test runs are not counted as people running the examples.
func exampleSource() string {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return ""
	}
	return downloadSource
}

// findAbove returns the working folder, or the first folder above it, joined
// with rel where that names a file. It returns "" when there is none.
func findAbove(rel string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
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
