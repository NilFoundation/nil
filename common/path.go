package common

import (
	"path/filepath"
	"runtime"
)

func GetAbsolutePath(file string) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("runtime.Caller failed")
	}
	path := filepath.Dir(filename)
	return filepath.Join(path, file)
}
