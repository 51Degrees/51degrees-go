/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// Test that GetFilePath passes over a Git LFS pointer file with the name, and
// a link file named after it, to find the file itself.
func TestGetFilePathPassesOverPointers(t *testing.T) {
	root := t.TempDir()
	name := "test.hash"
	// The search is in name order, so the pointer and the link come first.
	writeTestFile(t, filepath.Join(root, "a", name), lfsPointerStart+"v1\n"+
		"oid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\n"+
		"size 12345\n")
	writeTestFile(t, filepath.Join(root, "a", name+".link"),
		"https://example.com/test.hash\n")
	file := filepath.Join(root, "b", name)
	writeTestFile(t, file, "data")

	found, err := GetFilePath(root, []string{name})
	if err != nil {
		t.Fatalf("Error getting paths: %v", err)
	}
	if found != file {
		t.Errorf("Expected to find %s, but got %s instead.", file, found)
	}
}
