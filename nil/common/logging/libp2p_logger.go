package logging

import (
	libp2plog "github.com/ipfs/go-log/v2"
)

func SetLibp2pLogLevel(level string) error {
	if level == "" {
		return nil
	}

	return libp2plog.SetLogLevel("*", level)
}
