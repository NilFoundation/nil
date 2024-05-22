package types

import (
	"github.com/NilFoundation/nil/common"
	fastssz "github.com/ferranbt/fastssz"
	"github.com/rs/zerolog/log"
)

type VersionInfo struct {
	Version common.Hash `json:"version,omitempty"`
}

// interfaces
var (
	_ common.Hashable     = new(VersionInfo)
	_ fastssz.Marshaler   = new(VersionInfo)
	_ fastssz.Unmarshaler = new(VersionInfo)
)

func (m *VersionInfo) Hash() common.Hash {
	h, err := common.PoseidonSSZ(m)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't get version hash")
	}
	return h
}
