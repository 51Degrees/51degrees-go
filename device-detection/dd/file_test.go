package dd

import (
	"os"
	"path/filepath"
	"testing"
)

// Path for temp files and dirs used for testing
type testFilesPath struct {
	RootDir string
	SubDir  string
	File1   string
	File2   string
}

// Get File1 name without full path
func (fp *testFilesPath) GetFile1Name() string {
	return filepath.Base(fp.File1)
}

// Get File2 name without full path
func (fp *testFilesPath) GetFile2Name() string {
	return filepath.Base(fp.File2)
}

// Get RootDir name without full path
func (fp *testFilesPath) GetRootDirName() string {
	return filepath.Base(fp.RootDir)
}

// Get SubDir name without full path
func (fp *testFilesPath) GetSubDirName() string {
	return filepath.Base(fp.SubDir)
}

// Remove all temp files created
func (fp *testFilesPath) RemoveFiles() {
	os.RemoveAll(fp.RootDir)
}

// Create temp files for testing
func initTestFiles(t *testing.T) testFilesPath {
	rootDir, err := os.MkdirTemp("", "test_dir")
	if err != nil {
		t.Fatalf("Error creating test root directory: %v", err)
	}
	realRootDir, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		t.Fatalf("Error resolving symbolic link: %v", err)
	}

	osFile1, err := os.CreateTemp(rootDir, "test_file1")
	if err != nil {
		t.Fatalf("Error creating test file1: %v", err)
	}
	realFile1, err := filepath.EvalSymlinks(osFile1.Name())
	if err != nil {
		t.Fatalf("Error resolving symbolic link: %v", err)
	}

	subDir, err := os.MkdirTemp(rootDir, "sub_dir")
	if err != nil {
		t.Fatalf("Error creating test sub directory: %v", err)
	}
	realSubDir, err := filepath.EvalSymlinks(subDir)
	if err != nil {
		t.Fatalf("Error resolving symbolic link: %v", err)
	}

	osFile2, err := os.CreateTemp(subDir, "test_file2")
	if err != nil {
		t.Fatalf("Error creating test file2: %v", err)
	}
	realFile2, err := filepath.EvalSymlinks(osFile2.Name())
	if err != nil {
		t.Fatalf("Error resolving symbolic link: %v", err)
	}

	return testFilesPath{
		RootDir: realRootDir,
		SubDir:  realSubDir,
		File1:   realFile1,
		File2:   realFile2,
	}
}

// Test if GetFilePath can find file correctly with provided paths
func testGetFilePath(t *testing.T, pathInputs []string, fileName, result string) {
	for _, path := range pathInputs {
		resPath, err := GetFilePath(path, []string{fileName})
		if err != nil {
			t.Errorf("Error getting paths: %v", err)
		}
		if result != resPath {
			t.Errorf("Expected to have %s path from %s input, but got %s instead.\n",
				result,
				path,
				resPath)
		}
	}
}

// Test GetFilePath for realtive paths
func TestGetFilePathRelative(t *testing.T) {
	filesPath := initTestFiles(t)
	defer filesPath.RemoveFiles()

	rootDirName := filesPath.GetRootDirName()
	subDirName := filesPath.GetSubDirName()

	// Change working directory to RootDir where tmp files are stored in the system
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(filesPath.RootDir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}
	defer func() {
		err = os.Chdir(originalWD)
		if err != nil {
			t.Fatalf("Error changing working directory: %v", err)
		}
	}()

	// Paths for direct search
	path1Inputs := []string{
		".",
		"../" + rootDirName,
		subDirName + "/..",
	}
	// Paths for subdirectory search
	path2Inputs := []string{
		".",
		"./" + subDirName,
	}

	testGetFilePath(t, path1Inputs, filesPath.GetFile1Name(), filesPath.File1)
	testGetFilePath(t, path2Inputs, filesPath.GetFile2Name(), filesPath.File2)
}

// Test GetFilePath for absolute paths
func TestGetFilePathAbsolute(t *testing.T) {
	filesPath := initTestFiles(t)
	defer filesPath.RemoveFiles()

	rootDirName := filesPath.GetRootDirName()
	subDirName := filesPath.GetSubDirName()

	// Paths for direct search
	path1Inputs := []string{
		filesPath.RootDir,
		filesPath.RootDir + "/../" + rootDirName,
		filesPath.SubDir + "/..",
	}
	// Paths for subdirectory search
	path2Inputs := []string{
		filesPath.RootDir,
		filesPath.RootDir + "/" + subDirName,
	}

	testGetFilePath(t, path1Inputs, filesPath.GetFile1Name(), filesPath.File1)
	testGetFilePath(t, path2Inputs, filesPath.GetFile2Name(), filesPath.File2)
}
