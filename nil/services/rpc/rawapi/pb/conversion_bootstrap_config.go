package pb

import (
	"errors"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/network"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"gopkg.in/yaml.v3"
)

func (bc *BootstrapConfigResponse) PackProtoMessage(config *rpctypes.BootstrapConfig, err error) error {
	if err != nil {
		bc.Result = &BootstrapConfigResponse_Error{Error: new(Error).PackProtoMessage(err)}
		return nil
	}

	zeroStateConfigYaml, err := yaml.Marshal(config.ZeroStateConfig)
	if err != nil {
		return err
	}

	bc.Result = &BootstrapConfigResponse_Data{
		Data: &BootstrapConfig{
			NShards:             config.NShards,
			ZeroStateConfigYaml: string(zeroStateConfigYaml),
			DhtBootstrapPeers: slices.Collect(common.Transform(
				slices.Values(config.DhtBootstrapPeers),
				func(peer network.AddrInfo) string { return peer.String() })),
			BootstrapPeers: slices.Collect(common.Transform(
				slices.Values(config.BootstrapPeers),
				func(peer network.AddrInfo) string { return peer.String() })),
		},
	}

	return nil
}

func (bc *BootstrapConfigResponse) UnpackProtoMessage() (*rpctypes.BootstrapConfig, error) {
	switch res := bc.Result.(type) {
	case *BootstrapConfigResponse_Data:
		config := &rpctypes.BootstrapConfig{
			NShards: res.Data.GetNShards(),
		}
		if err := yaml.Unmarshal([]byte(res.Data.GetZeroStateConfigYaml()), &config.ZeroStateConfig); err != nil {
			return nil, err
		}
		config.DhtBootstrapPeers = make(network.AddrInfoSlice, len(res.Data.GetDhtBootstrapPeers()))
		for i, peer := range res.Data.GetDhtBootstrapPeers() {
			var addrInfo network.AddrInfo
			if err := addrInfo.Set(peer); err != nil {
				return nil, err
			}
			config.DhtBootstrapPeers[i] = addrInfo
		}
		config.BootstrapPeers = make(network.AddrInfoSlice, len(res.Data.GetBootstrapPeers()))
		for i, peer := range res.Data.GetBootstrapPeers() {
			var addrInfo network.AddrInfo
			if err := addrInfo.Set(peer); err != nil {
				return nil, err
			}
			config.BootstrapPeers[i] = addrInfo
		}
		return config, nil

	case *BootstrapConfigResponse_Error:
		return nil, res.Error.UnpackProtoMessage()
	}
	return nil, errors.New("unexpected response type")
}
