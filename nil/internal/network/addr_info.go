package network

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

type AddrInfo peer.AddrInfo

func (a *AddrInfo) Set(value string) error {
	addr, err := peer.AddrInfoFromString(value)
	if err != nil {
		return err
	}
	*a = AddrInfo(*addr)
	return nil
}

func (a *AddrInfo) String() string {
	mu, err := peer.AddrInfoToP2pAddrs((*peer.AddrInfo)(a))
	if err != nil {
		return err.Error()
	}
	values := make([]string, len(mu))
	for i, val := range mu {
		values[i] = val.String()
	}
	str, err := common.WriteAsCSV(values)
	if err != nil {
		return err.Error()
	}
	return str
}

func (a *AddrInfo) Type() string {
	return "AddrInfo"
}

func (a AddrInfo) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *AddrInfo) UnmarshalText(text []byte) error {
	return a.Set(string(text))
}

type AddrInfoSlice = common.PValueSlice[*AddrInfo, AddrInfo]

func ToLibP2pAddrInfoSlice(s AddrInfoSlice) []peer.AddrInfo {
	res := make([]peer.AddrInfo, len(s))
	for i, a := range s {
		res[i] = peer.AddrInfo(a)
	}
	return res
}
