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

// ConfigAccessorRo provides read access to config params stored in a Merkle Patricia Trie.
type ConfigAccessorRo struct {
	trie *mpt.Reader
}

// ConfigAccessor provides read/write access to config params stored in a Merkle Patricia Trie.
type ConfigAccessor struct {
	*ConfigAccessorRo
	data       map[string][]byte
	allowWrite bool
}

// IConfigParam is an interface that all config params must implement.
type IConfigParam interface {
	ssz.Unmarshaler

	Name() string
	Accessor() *ParamAccessor
}

// IConfigParamPointer is an interface that allows to avoid error like:
// `... does not satisfy IConfigParam (method ... has pointer receiver)`
type IConfigParamPointer[T any] interface {
	*T
	IConfigParam
}

// ParamAccessor provides functions to work with the concrete parameter. Such as read/write parameter from configuration
// and pack/unpack from Solidity data.
type ParamAccessor struct {
	getRo  func(c *ConfigAccessorRo) (any, error)
	get    func(c *ConfigAccessor) (any, error)
	set    func(c *ConfigAccessor, v any) error
	pack   func(v any) ([]byte, error)
	unpack func(data []byte) (any, error)
}

// NewConfigAccessor creates a new ConfigAccessor fetching MPT itself.
func NewConfigAccessor(tx db.RoTx, shardId types.ShardId, mainShardHash *common.Hash) (*ConfigAccessor, error) {
	trie, err := getConfigTrie(tx, mainShardHash)
	if err != nil {
		return nil, err
	}
	return &ConfigAccessor{
		&ConfigAccessorRo{trie},
		make(map[string][]byte),
		shardId.IsMainShard(),
	}, nil
}

// NewConfigAccessor creates a new ConfigAccessor fetching MPT itself.
func NewConfigAccessorRo(tx db.RoTx, mainShardHash *common.Hash) (*ConfigAccessorRo, error) {
	trie, err := getConfigTrie(tx, mainShardHash)
	if err != nil {
		return nil, err
	}
	return &ConfigAccessorRo{trie}, nil
}

// UpdateConfigTrie updates the config trie with the current state of the ConfigAccessor.
func (c *ConfigAccessor) UpdateConfigTrie(tx db.RwTx, root common.Hash) (common.Hash, error) {
	if len(c.data) == 0 {
		return root, nil
	}
	trie := mpt.NewDbMPT(tx, types.MainShardId, db.ConfigTrieTable)
	trie.SetRootHash(root)
	for k, v := range c.data {
		if err := trie.Set([]byte(k), v); err != nil {
			return common.EmptyHash, err
		}
	}
	return trie.RootHash(), nil
}

// GetParam retrieves the value of the specified config param.
func (c *ConfigAccessorRo) GetParam(name string) (any, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.getRo(c)
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
func (c *ConfigAccessorRo) UnpackSolidity(name string, data []byte) (any, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.unpack(data)
}

// PackSolidity packs the specified config parameter into a byte slice.
func (c *ConfigAccessorRo) PackSolidity(name string, v any) ([]byte, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.pack(v)
}

func (c *ConfigAccessorRo) GetParamValidators() (*ParamValidators, error) {
	return GetParamRo[ParamValidators](c)
}

func (c *ConfigAccessor) GetParamValidators() (*ParamValidators, error) {
	return GetParam[ParamValidators](c)
}

func (c *ConfigAccessor) SetParamValidators(params *ParamValidators) error {
	return SetParam[ParamValidators](c, params)
}

func (c *ConfigAccessorRo) GetParamGasPrice() (*ParamGasPrice, error) {
	return GetParamRo[ParamGasPrice](c)
}

func (c *ConfigAccessor) GetParamGasPrice() (*ParamGasPrice, error) {
	return GetParam[ParamGasPrice](c)
}

func (c *ConfigAccessor) SetParamGasPrice(params *ParamGasPrice) error {
	return SetParam[ParamGasPrice](c, params)
}

// GetParamRo retrieves the value of the specified config param from trie.
func GetParamRo[T any, paramPtr IConfigParamPointer[T]](c *ConfigAccessorRo) (*T, error) {
	var res paramPtr = new(T)
	data, err := c.trie.Get([]byte(res.Name()))
	if err != nil {
		return nil, fmt.Errorf("failed to read config param: %w", err)
	}
	if err = res.UnmarshalSSZ(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config param: %w", err)
	}
	return res, nil
}

// GetParam retrieves the value of the specified config param from in-memory data or trie.
func GetParam[T any, paramPtr IConfigParamPointer[T]](c *ConfigAccessor) (*T, error) {
	var res paramPtr = new(T)
	data, ok := c.data[res.Name()]
	if !ok {
		return GetParamRo[T, paramPtr](c.ConfigAccessorRo)
	}
	if err := res.UnmarshalSSZ(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config param: %w", err)
	}
	return res, nil
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
			c.data[name] = data
			return nil
		}
		return errors.New("type does not implement ssz.Marshaler")
	}
	return errors.New("type does not implement types.IConfigParam")
}

func getConfigTrie(tx db.RoTx, mainShardHash *common.Hash) (*mpt.Reader, error) {
	configTree := mpt.NewDbReader(tx, types.MainShardId, db.ConfigTrieTable)
	lastBlock := mainShardHash == nil || mainShardHash.Empty()

	var mainChainBlock *types.Block
	var err error

	if lastBlock {
		mainChainBlock, _, err = db.ReadLastBlock(tx, types.MainShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
	} else {
		if mainChainBlock, err = db.ReadBlock(tx, types.MainShardId, *mainShardHash); err != nil {
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
