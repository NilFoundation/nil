package bridgecontract

import (
	"context"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/sync/errgroup"
)

type bytes32 = [32]uint8

type BridgeState struct {
	L2toL1Root    common.Hash
	L1MessageHash common.Hash
	DepositNonce  *big.Int
}

func newBridgeState(
	l2toL1Root common.Hash,
	l1MessageHash common.Hash,
	depositNonce *big.Int,
) BridgeState {
	return BridgeState{
		L2toL1Root:    l2toL1Root,
		L1MessageHash: l1MessageHash,
		DepositNonce:  depositNonce,
	}
}

var BridgeStateEmpty = newBridgeState(common.EmptyHash, common.EmptyHash, big.NewInt(0))

type BridgeStateGetter interface {
	GetBridgeState(ctx context.Context, blockHash common.Hash) (*BridgeState, error)
}

type bridgeStateGetter struct {
	nilClient    client.Client
	contractAddr types.Address
	abi          *abi.ABI
	logger       logging.Logger
}

func NewBridgeStateGetter(
	nilClient client.Client,
	l2ContractAddr types.Address,
	logger logging.Logger,
) *bridgeStateGetter {
	return &bridgeStateGetter{
		nilClient:    nilClient,
		contractAddr: l2ContractAddr,
		abi:          GetL2BridgeStateGetterABI(),
		logger:       logger,
	}
}

func (b *bridgeStateGetter) GetBridgeState(ctx context.Context, blockHash common.Hash) (*BridgeState, error) {
	exists, err := b.contactExistsAtBlock(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	if !exists {
		b.logger.Warn().
			Any(logging.FieldBlockHash, blockHash).
			Msg("L2 bridge contract does not exist at specified block, empty state will be returned")
		return &BridgeStateEmpty, nil
	}

	eg, gCtx := errgroup.WithContext(ctx)

	var l1MessageHash common.Hash
	eg.Go(func() error {
		const method = "l1MessageHash"
		ret, err := callContract[bytes32](gCtx, b.nilClient, blockHash, b.contractAddr, b.abi, method)
		if err != nil {
			return b.callError(method, blockHash, err)
		}
		l1MessageHash = common.BytesToHash(ret[:])
		return nil
	})

	var l2ToL1Root common.Hash
	eg.Go(func() error {
		const method = "getL2ToL1Root"
		ret, err := callContract[bytes32](gCtx, b.nilClient, blockHash, b.contractAddr, b.abi, method)
		if err != nil {
			return b.callError(method, blockHash, err)
		}
		l2ToL1Root = common.BytesToHash(ret[:])
		return nil
	})

	var depositNonce *big.Int
	eg.Go(func() error {
		const method = "getLatestDepositNonce"
		ret, err := callContract[*big.Int](gCtx, b.nilClient, blockHash, b.contractAddr, b.abi, method)
		if err != nil {
			return b.callError(method, blockHash, err)
		}
		depositNonce = ret
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	ret := newBridgeState(l2ToL1Root, l1MessageHash, depositNonce)
	return &ret, nil
}

func (b *bridgeStateGetter) contactExistsAtBlock(ctx context.Context, blockId any) (bool, error) {
	contractCode, err := b.nilClient.GetCode(ctx, b.contractAddr, blockId)
	if err != nil {
		return false, fmt.Errorf("failed to check if L2 bridge contract exists at block %v: %w", blockId, err)
	}
	return len(contractCode) != 0, nil
}

func (*bridgeStateGetter) callError(method string, blockHash common.Hash, cause error) error {
	return fmt.Errorf("method %s failed, blockHash=%s: %w", method, blockHash, cause)
}

func callContract[Ret any](
	ctx context.Context,
	c client.Client,
	blockID any,
	addr types.Address,
	abi *abi.ABI,
	method string,
	args ...any,
) (Ret, error) {
	var ret Ret
	calldata, err := abi.Pack(method, args...)
	if err != nil {
		return ret, err
	}

	callArgs := &jsonrpc.CallArgs{
		Data: (*hexutil.Bytes)(&calldata),
		To:   addr,
		Fee:  types.NewFeePackFromGas(100_000), // TODO(oclaw) which value to use here?
	}

	callRes, err := c.Call(ctx, callArgs, blockID, nil)
	if err != nil {
		return ret, err
	}

	res, err := abi.Unpack(method, callRes.Data)
	if err != nil {
		return ret, err
	}

	if len(res) != 1 {
		return ret, fmt.Errorf("expected single return value, got %d", len(res))
	}

	var ok bool
	ret, ok = res[0].(Ret)
	if !ok {
		return ret, fmt.Errorf("type mismatch for return value of method %s: expected %T got %T", method, ret, res[0])
	}

	return ret, nil
}
