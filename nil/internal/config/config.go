package config

import (
	"context"
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

type ConfigAccessor interface {
	GetParamData(name string) ([]byte, error)
	SetParamData(name string, data []byte) error
	Commit(tx db.RwTx, root common.Hash) (common.Hash, error)
}

// configAccessorImpl provides read/write access to config params that were read from Config's MPT.
type configAccessorImpl struct {
	readData  map[string][]byte
	writeData map[string][]byte
}

// configReader provides read-only access to config params that were read from Config's MPT.
// Unlike configAccessorImpl it doesn't read all params during initialization, it reads it only on demand.
// Also, it doesn't allow writing params.
type configReader struct {
	trie *mpt.Reader
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
	get    func(c ConfigAccessor) (any, error)
	set    func(c ConfigAccessor, v any) error
	pack   func(v any) ([]byte, error)
	unpack func(data []byte) (any, error)
}

func NewConfigReader(tx db.RoTx, mainShardHash *common.Hash) (ConfigAccessor, error) {
	trie, err := getConfigTrie(tx, mainShardHash)
	if err != nil {
		return nil, err
	}
	return &configReader{trie}, nil
}

type ConfigAccessorStub struct{}

var _ ConfigAccessor = (*ConfigAccessorStub)(nil)

func (c *ConfigAccessorStub) GetParamData(name string) ([]byte, error) {
	return nil, errors.New("stub config accessor should not be called")
}

func (c *ConfigAccessorStub) SetParamData(name string, data []byte) error {
	return errors.New("stub config accessor should not be called")
}

func (c *ConfigAccessorStub) GetParam(name string) (any, error) {
	return nil, errors.New("stub config accessor should not be called")
}

func (c *ConfigAccessorStub) SetParam(name string, value any) error {
	return errors.New("stub config accessor should not be called")
}

func (c *ConfigAccessorStub) Commit(tx db.RwTx, root common.Hash) (common.Hash, error) {
	return common.EmptyHash, nil
}

func GetStubAccessor() ConfigAccessor {
	return &ConfigAccessorStub{}
}

// NewConfigAccessor creates a new configAccessorImpl reading the whole trie from the MPT.
func NewConfigAccessor(ctx context.Context, db db.DB, mainShardHash *common.Hash) (ConfigAccessor, error) {
	tx, err := db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create read-only transaction: %w", err)
	}
	defer tx.Rollback()

	trie, err := getConfigTrie(tx, mainShardHash)
	if err != nil {
		return nil, err
	}
	data := make(map[string][]byte)
	for k, v := range trie.Iterate() {
		data[string(k)] = v
	}
	return &configAccessorImpl{
		data,
		make(map[string][]byte),
	}, nil
}

// Commit updates the config trie with the current state of the configAccessorImpl.
func (c *configAccessorImpl) Commit(tx db.RwTx, root common.Hash) (common.Hash, error) {
	if len(c.writeData) == 0 {
		return root, nil
	}
	trie := mpt.NewDbMPT(tx, types.MainShardId, db.ConfigTrieTable)
	trie.SetRootHash(root)
	for k, v := range c.writeData {
		if err := trie.Set([]byte(k), v); err != nil {
			return common.EmptyHash, err
		}
	}
	return trie.RootHash(), nil
}

func (c *configReader) GetParamData(name string) ([]byte, error) {
	return c.trie.Get([]byte(name))
}

func (c *configReader) SetParamData(name string, data []byte) error {
	return errors.New("call `SetParamData` for read-only config accessor")
}

func (c *configReader) Commit(tx db.RwTx, root common.Hash) (common.Hash, error) {
	return common.EmptyHash, errors.New("call `Commit` for read-only config accessor")
}

func (c *configAccessorImpl) GetParamData(name string) ([]byte, error) {
	data, ok := c.writeData[name]
	if !ok {
		data, ok = c.readData[name]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
		}
	}
	return data, nil
}

func (c *configAccessorImpl) SetParamData(name string, data []byte) error {
	c.writeData[name] = data
	return nil
}

// GetParam retrieves the value of the specified config param.
func GetParam(c ConfigAccessor, name string) (any, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.get(c)
}

// SetParam sets the value of the specified config param.
func SetParam(c ConfigAccessor, name string, v any) error {
	param, ok := ParamsMap[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.set(c, v)
}

// UnpackSolidity unpacks the given data into the specified config param.
func UnpackSolidity(name string, data []byte) (any, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.unpack(data)
}

// PackSolidity packs the specified config parameter into a byte slice.
func PackSolidity(name string, v any) ([]byte, error) {
	param, ok := ParamsMap[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrParamNotFound, name)
	}
	return param.pack(v)
}

// getParamImpl retrieves the value of the specified config param from in-memory data or trie.
func getParamImpl[T any, paramPtr IConfigParamPointer[T]](c ConfigAccessor) (*T, error) {
	var res paramPtr = new(T)
	data, err := c.GetParamData(res.Name())
	if err != nil {
		return nil, err
	}
	if err := res.UnmarshalSSZ(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config param: %w", err)
	}
	return res, nil
}

// setParamImpl sets the value of the specified config param.
func setParamImpl[T any](c ConfigAccessor, obj *T) error {
	if configParam, ok := any(obj).(IConfigParam); ok {
		name := configParam.Name()
		if marshaler, ok := any(obj).(ssz.Marshaler); ok {
			data, err := marshaler.MarshalSSZ()
			if err != nil {
				return fmt.Errorf("failed to marshal config param %s: %w", name, err)
			}
			return c.SetParamData(name, data)
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

// packSolidityImpl packs the specified config param into a byte slice.
func packSolidityImpl[T any](obj *T) ([]byte, error) {
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

// unpackSolidityImpl unpacks the given data into the specified config param.
func unpackSolidityImpl[T any](data []byte) (*T, error) {
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
