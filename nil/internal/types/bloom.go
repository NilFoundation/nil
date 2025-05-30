package types

import (
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type Bloom = ethtypes.Bloom

var BytesToBloom = ethtypes.BytesToBloom

func CreateBloom(receipts Receipts) Bloom {
	var bin Bloom
	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			bin.Add(log.Address.Bytes())
			for _, topic := range log.Topics {
				bin.Add(topic[:])
			}
		}
	}
	return bin
}

// LogsBloom returns the bloom bytes for the given logs
func LogsBloom(logs []*Log) []byte {
	var bin Bloom
	for _, log := range logs {
		bin.Add(log.Address.Bytes())
		for i := range len(log.Topics) {
			bin.Add(log.Topics[i][:])
		}
	}
	return bin[:]
}
