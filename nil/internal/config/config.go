package config

import (
	"errors"
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var (
	ParamsMap        = make(map[string]*ParamAccessor)
	ErrParamNotFound = errors.New("param not found")
)

func init() {
	for _, param := range ParamsList {
		ParamsMap[param.Name()] = param.Accessor()
	}
}

// ConfigAccessor provides access to config params stored in a Merkle Patricia Trie.
type ConfigAccessor struct {
	trie       *mpt.MerklePatriciaTrie
	allowWrite bool
}

// IConfigParam is an interface that all config params must implement.
type IConfigParam interface {
	Name() string
	Accessor() *ParamAccessor
}

// ParamAccessor provides functions to work with the concrete parameter. Such as read/write parameter from configuration
// and pack/unpack from Solidity data.
type ParamAccessor struct {
	get    func(c *ConfigAccessor) (any, error)
	set    func(c *ConfigAccessor, v any) error
	pack   func(c *ConfigAccessor, v any) ([]byte, error)
	unpack func(c *ConfigAccessor, data []byte) (any, error)
}

// NewConfigAccessor creates a new ConfigAccessor fetching MPT itself.
func NewConfigAccessor(tx db.RwTx, shardId types.ShardId, hash *common.Hash) (*ConfigAccessor, error) {
	trie, err := getConfigTrie(tx, shardId, hash)
	if err != nil {
		return nil, err
	}
	return &ConfigAccessor{trie, shardId.IsMainShard()}, nil
}

// NewConfigAccessorFromMpt creates a new ConfigAccessor from an existing MPT.
func NewConfigAccessorFromMpt(trie *mpt.MerklePatriciaTrie, shardId types.ShardId) *ConfigAccessor {
	return &ConfigAccessor{trie, shardId.IsMainShard()}
}

func (c *ConfigAccessor) GetRootHash() common.Hash {
	return c.trie.RootHash()
}

// GetParam retrieves the value of the specified config param.
func (c *ConfigAccessor) GetParam(name string) (any, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.get(c)
}

// SetParam sets the value of the specified config param.
func (c *ConfigAccessor) SetParam(name string, v any) error {
	param, ok := ParamsMap[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.set(c, v)
}

// UnpackSolidity unpacks the given data into the specified config param.
func (c *ConfigAccessor) UnpackSolidity(name string, data []byte) (any, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.unpack(c, data)
}

// PackSolidity packs the specified config parameter into a byte slice.
func (c *ConfigAccessor) PackSolidity(name string, v any) ([]byte, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.pack(c, v)
}

func (c *ConfigAccessor) GetParamValidators() (*ParamValidators, error) {
	return GetParam[ParamValidators](c)
}

func (c *ConfigAccessor) SetParamValidators(params *ParamValidators) error {
	return SetParam[ParamValidators](c, params)
}

func (c *ConfigAccessor) GetParamGasPrice() (*ParamGasPrice, error) {
	return GetParam[ParamGasPrice](c)
}

func (c *ConfigAccessor) SetParamGasPrice(params *ParamGasPrice) error {
	return SetParam[ParamGasPrice](c, params)
}

// GetParam retrieves the value of the specified config param.
func GetParam[T any](c *ConfigAccessor) (*T, error) {
	res := new(T)
	if configParam, ok := any(res).(IConfigParam); ok {
		data, err := c.trie.Get([]byte(configParam.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read config param: %w", err)
		}
		if unmarshaler, ok := any(res).(ssz.Unmarshaler); ok {
			err = unmarshaler.UnmarshalSSZ(data)
			return res, err
		}
		return nil, errors.New("type does not implement ssz.Unmarshaler")
	}
	return nil, errors.New("type does not implement types.IConfigParam")
}

// SetParam sets the value of the specified config param.
func SetParam[T any](c *ConfigAccessor, obj *T) error {
	if !c.allowWrite {
		return errors.New("ConfigAccessor is read-only for non-main shard contracts")
	}
	if configParam, ok := any(obj).(IConfigParam); ok {
		name := configParam.Name()
		if marshaler, ok := any(obj).(ssz.Marshaler); ok {
			data, err := marshaler.MarshalSSZ()
			if err != nil {
				return fmt.Errorf("failed to marshal config param %s: %w", name, err)
			}
			return c.trie.Set([]byte(name), data)
		} else {
			return errors.New("type does not implement ssz.Marshaler")
		}
	}
	return errors.New("type does not implement types.IConfigParam")
}

// getConfigTrie retrieves the configuration tree from the database.
func getConfigTrie(tx db.RwTx, shardId types.ShardId, hash *common.Hash) (*mpt.MerklePatriciaTrie, error) {
	configTree := mpt.NewDbMPT(tx, shardId, db.ConfigTrieTable)
	lastBlock := hash == nil || *hash == common.EmptyHash

	var mainChainBlock *types.Block
	var err error

	if shardId == types.MainShardId {
		if lastBlock {
			mainChainBlock, err = db.ReadLastBlock(tx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
		} else {
			mainChainBlock, err = db.ReadBlock(tx, shardId, *hash)
			if err != nil {
				return nil, err
			}
		}
	} else {
		var block *types.Block
		if lastBlock {
			block, err = db.ReadLastBlock(tx, shardId)
			if err != nil {
				return nil, err
			}
		} else {
			block, err = db.ReadBlock(tx, shardId, *hash)
			if err != nil {
				return nil, err
			}
		}
		mainChainBlock, err = db.ReadBlock(tx, shardId, block.MainChainHash)
		if err != nil {
			return nil, err
		}
	}

	if mainChainBlock != nil {
		configTree.SetRootHash(mainChainBlock.ConfigRoot)
	}

	return configTree, nil
}

// PackSolidity packs the specified config param into a byte slice.
func PackSolidity[T any](obj *T) ([]byte, error) {
	precompileAbi, err := contracts.GetAbi(contracts.NameNilConfigAbi)
	if err != nil {
		return nil, err
	}
	var paramAbi abi.Arguments
	if configParam, ok := any(new(T)).(IConfigParam); ok {
		m, ok := precompileAbi.Methods[configParam.Name()]
		if !ok {
			return nil, errors.New("method not found")
		}
		paramAbi = m.Inputs
	} else {
		return nil, errors.New("type does not implement types.IConfigParam")
	}

	data, err := paramAbi.Pack(obj)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// UnpackSolidity unpacks the given data into the specified config param.
func UnpackSolidity[T any](data []byte) (*T, error) {
	precompileAbi, err := contracts.GetAbi(contracts.NameNilConfigAbi)
	if err != nil {
		return nil, err
	}
	var paramAbi abi.Arguments
	obj := new(T)
	if configParam, ok := any(obj).(IConfigParam); ok {
		m, ok := precompileAbi.Methods[configParam.Name()]
		if !ok {
			return nil, errors.New("method not found")
		}
		paramAbi = m.Inputs
	} else {
		return nil, errors.New("type does not implement types.IConfigParam")
	}

	unpacked, err := paramAbi.Unpack(data)
	if err != nil {
		return nil, err
	}
	v := abi.ConvertType(unpacked[0], obj)
	res, ok := v.(*T)
	if !ok {
		return nil, errors.New("failed to unpack")
	}

	return res, nil
}
