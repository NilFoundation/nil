//go:build test

package rpc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const tmpDir = "/tmp"

func makeSockPathImpl(name string, filename string, postprocess func(dir string, err error)) string {
	dirName := strings.ReplaceAll(name, "/", "_")
	maxDirNameLen := 108 - 1 - // https://github.com/golang/go/blob/go1.24.0/src/syscall/ztypes_linux_amd64.go#L172
		len(tmpDir) - //
		1 - // "/"
		1 - // "_"
		10 - // max random 32bit number digits
		1 - // "/"
		len(filename)

	if len(dirName) >= maxDirNameLen {
		dirName = dirName[:maxDirNameLen]
	}
	dir, err := os.MkdirTemp(tmpDir, dirName+"_*")
	postprocess(dir, err)
	return filepath.Join(dir, filename)
}

func makeSockPath(t *testing.T, filename string) string {
	t.Helper()

	return makeSockPathImpl(
		t.Name(),
		filename,
		func(dir string, err error) {
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
		})
}

func GetSockPath(t *testing.T) string {
	t.Helper()
	return "unix://" + makeSockPath(t, "httpd.sock")
}

func GetSockPathIdx(t *testing.T, idx int) string {
	t.Helper()
	return "unix://" + makeSockPath(t, fmt.Sprintf("httpd%d.sock", idx))
}

func GetSockPathService(t *testing.T, service string) string {
	t.Helper()
	return "unix://" + makeSockPath(t, fmt.Sprintf("httpd_%s.sock", service))
}
