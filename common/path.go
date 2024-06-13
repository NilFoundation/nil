package common

import (
	"path/filepath"
	"runtime"

	"github.com/NilFoundation/nil/common/check"
)

func GetAbsolutePath(file string) string {
	_, filename, _, ok := runtime.Caller(1)
	check.PanicIfNot(ok)

	path := filepath.Dir(filename)
	return filepath.Join(path, file)
}
