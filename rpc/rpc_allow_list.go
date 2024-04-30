package rpc

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/NilFoundation/nil/rpc/transport"
)

type allowListFile struct {
	Allow transport.AllowList `json:"allow"`
}

func parseAllowListForRPC(path string) (transport.AllowList, error) {
	path = strings.TrimSpace(path)
	if path == "" { // no file is provided
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		file.Close() //nolint: errcheck
	}()

	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var allowListFileObj allowListFile

	err = json.Unmarshal(fileContents, &allowListFileObj)
	if err != nil {
		return nil, err
	}

	return allowListFileObj.Allow, nil
}
