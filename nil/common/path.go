package common

import (
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/nil/common/check"
)

func GetAbsolutePath(file string) string {
	path, err := os.Getwd()
	check.PanicIfErr(err)
	abs, err := filepath.Abs(filepath.Join(path, file))
	check.PanicIfErr(err)
	return abs
}
