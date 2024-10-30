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

func getSockDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("/tmp", strings.ReplaceAll(t.Name(), "/", "_")+"_*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func GetSockPath(t *testing.T) string {
	t.Helper()
	return "unix://" + filepath.Join(getSockDir(t), "httpd.sock")
}

func GetSockPathIdx(t *testing.T, idx int) string {
	t.Helper()
	return fmt.Sprintf("unix://%s/httpd%d.sock", getSockDir(t), idx)
}

func GetSockPathService(t *testing.T, service string) string {
	t.Helper()
	return fmt.Sprintf("unix://%s/httpd_%s.sock", getSockDir(t), service)
}
