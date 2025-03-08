package config

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/crypto/bls"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog/log"
)

const ValidatorPubkeySize = 128

const (
	NameValidators = "curr_validators"
	NameGasPrice   = "gas_price"
	NameL1Block    = "l1block"
)

var ParamsList = []IConfigParam{
	new(ParamValidators),
	new(ParamGasPrice),
	new(ParamL1BlockInfo),
}

type Pubkey [ValidatorPubkeySize]byte

func (k Pubkey) MarshalText() ([]byte, error) {
	return hexutil.Bytes(k[:]).MarshalText()
}

func (k *Pubkey) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Pubkey", input, k[:])
}

func InitParams(accessor ConfigAccessor) {
	for _, p := range ParamsList {
		data, err := p.MarshalSSZ()
		check.PanicIfErr(err)
		err = accessor.SetParamData(p.Name(), data)
		check.PanicIfErr(err)
	}
}

// This is a workaround for fastssz bug where it doesn't add import of `types` package to generated code.
// Adding this struct solves the issue. It can be removed once something other from `types` package will be used in the
// following structs.
type WorkaroundToImportTypes struct {
	Tmp types.TransactionIndex
}

var ErrParamCastFailed = errors.New("input object cannot be cast to Param")

type ListValidators struct {
	List []ValidatorInfo `json:"list" ssz-max:"4096" yaml:"list"`
}

type ParamValidators struct {
	Validators []ListValidators `json:"validators" ssz-max:"4096" yaml:"validators"`
}

type ValidatorInfo struct {
	PublicKey         Pubkey        `json:"pubKey" yaml:"pubKey" ssz-size:"128"`
	WithdrawalAddress types.Address `json:"withdrawalAddress" yaml:"withdrawalAddress"`
}

var _ IConfigParam = new(ParamValidators)

func (p *ParamValidators) Name() string {
	return NameValidators
}

func (p *ParamValidators) Accessor() *ParamAccessor {
	return CreateAccessor[ParamValidators]()
}

type ParamGasPrice struct {
	Shards []types.Uint256 `json:"shards" ssz-max:"4096" yaml:"shards"`
}

var _ IConfigParam = new(ParamGasPrice)

func (p *ParamGasPrice) Name() string {
	return NameGasPrice
}

func (p *ParamGasPrice) Accessor() *ParamAccessor {
	return CreateAccessor[ParamGasPrice]()
}

type ParamL1BlockInfo struct {
	Number      uint64        `json:"number" yaml:"number"`
	Timestamp   uint64        `json:"timestamp" yaml:"timestamp"`
	BaseFee     types.Uint256 `json:"baseFee" yaml:"baseFee"`
	BlobBaseFee types.Uint256 `json:"blobBaseFee" yaml:"blobBaseFee"`
	Hash        common.Hash   `json:"hash" yaml:"hash"`
}

var _ IConfigParam = new(ParamL1BlockInfo)

func (p *ParamL1BlockInfo) Name() string {
	return NameL1Block
}

func (p *ParamL1BlockInfo) Accessor() *ParamAccessor {
	return CreateAccessor[ParamL1BlockInfo]()
}

func CreateAccessor[T any, paramPtr IConfigParamPointer[T]]() *ParamAccessor {
	return &ParamAccessor{
		func(c ConfigAccessor) (any, error) {
			return getParamImpl[T, paramPtr](c)
		},
		func(c ConfigAccessor, v any) error {
			if param, ok := v.(*T); ok {
				return setParamImpl[T](c, param)
			}
			return ErrParamCastFailed
		},
		func(v any) ([]byte, error) {
			if param, ok := v.(*T); ok {
				return packSolidityImpl[T](param)
			}
			return nil, ErrParamCastFailed
		},
		func(data []byte) (any, error) { return unpackSolidityImpl[T](data) },
	}
}

func GetParamValidators(c ConfigAccessor) (*ParamValidators, error) {
	return getParamImpl[ParamValidators](c)
}

func NewConfigAccessorFromBlock(ctx context.Context, database db.DB, block *types.Block, shardId types.ShardId) (ConfigAccessor, error) {
	tx, err := database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return NewConfigAccessorFromBlockWithTx(tx, block, shardId)
}

func NewConfigAccessorFromBlockWithTx(tx db.RoTx, block *types.Block, shardId types.ShardId) (ConfigAccessor, error) {
	var mainShardHash *common.Hash
	if block != nil {
		h := block.GetMainShardHash(shardId)
		mainShardHash = &h
		// It's the first block of main shard. Use configuration from itself.
		if mainShardHash.Empty() {
			h := block.Hash(types.MainShardId)
			mainShardHash = &h
		}
	}

	c, err := NewConfigAccessorTx(tx, mainShardHash)
	if err != nil {
		return nil, err
	}

	if mainShardHash != nil {
		if _, err := db.ReadBlock(tx, types.MainShardId, *mainShardHash); errors.Is(err, db.ErrKeyNotFound) {
			// It is possible that the needed main chain block has not arrived yet, or that this one is some byzantine block.
			// Because right now the config is actually constant, we can use whatever version we like in this case,
			// so we use the latest accessible config.
			// TODO(@isergeyam): create some subscription mechanism that will handle this correctly.
			log.Warn().
				Stringer(logging.FieldBlockNumber, block.Id).
				Stringer(logging.FieldBlockMainChainHash, mainShardHash).
				Msg("Main chain block not found, using the latest accessible config")
			return NewConfigAccessorTx(tx, nil)
		}
	}
	return c, err
}

type PublicKeyMap struct {
	m    map[Pubkey]uint32
	keys []bls.PublicKey
}

func NewPublicKeyMap() *PublicKeyMap {
	return &PublicKeyMap{m: make(map[Pubkey]uint32)}
}

func (m *PublicKeyMap) Keys() []bls.PublicKey {
	return m.keys
}

func (m *PublicKeyMap) Find(key Pubkey) (uint32, bool) {
	i, ok := m.m[key]
	return i, ok
}

func (m *PublicKeyMap) Len() int {
	return len(m.keys)
}

func (m *PublicKeyMap) add(key Pubkey) error {
	index := uint32(len(m.keys))
	m.m[key] = index
	pk, err := bls.PublicKeyFromBytes(key[:])
	if err != nil {
		return err
	}
	m.keys = append(m.keys, pk)
	return nil
}

func CreateValidatorsPublicKeyMap(validators []ValidatorInfo) (*PublicKeyMap, error) {
	m := NewPublicKeyMap()
	for _, v := range validators {
		if err := m.add(v.PublicKey); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func SetParamValidators(c ConfigAccessor, params *ParamValidators) error {
	return setParamImpl(c, params)
}

func GetParamGasPrice(c ConfigAccessor) (*ParamGasPrice, error) {
	return getParamImpl[ParamGasPrice](c)
}

func SetParamGasPrice(c ConfigAccessor, params *ParamGasPrice) error {
	return setParamImpl(c, params)
}

func GetParamL1Block(c ConfigAccessor) (*ParamL1BlockInfo, error) {
	return getParamImpl[ParamL1BlockInfo](c)
}

func SetParamL1Block(c ConfigAccessor, params *ParamL1BlockInfo) error {
	return setParamImpl(c, params)
}

func GetParamNShards(c ConfigAccessor) (uint32, error) {
	param, err := getParamImpl[ParamGasPrice](c)
	if err != nil {
		return 0, err
	}
	return uint32(len(param.Shards)), nil
}
