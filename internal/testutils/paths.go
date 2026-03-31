package testutils

import (
	"path/filepath"
	"runtime"
)

func GetRootDir() string {
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed in GetRootDir")
	}

	// Adjust the number of ".." based on how deep this file is
	return filepath.Join(filepath.Dir(b), "../..")
}

// TestFilePath constructs the absolute path to a test file
// located in the "testdata" directory in the root of the project.
func TestFilePath(filename string) string {
	return filepath.Join(GetRootDir(), "testdata", filename)
}
