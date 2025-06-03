package bridgecontract

import (
	"context"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/sync/errgroup"
)

type bytes32 = [32]uint8

type BridgeState struct {
	L2toL1Root    *big.Int
	DepositNonce  *big.Int
	L1MessageHash *big.Int
}

type BridgeStateGetter interface {
	GetBridgeState(ctx context.Context, blockHash common.Hash) (*BridgeState, error)
}

type bridgeStateGetter struct {
	nilClient    client.Client
	contractAddr types.Address
	abi          *abi.ABI
}

func NewBridgeStateGetter(
	nilClient client.Client,
	l2ContractAddr types.Address,
) *bridgeStateGetter {
	return &bridgeStateGetter{
		nilClient:    nilClient,
		contractAddr: l2ContractAddr,
		abi:          GetL2BridgeStateGetterABI(),
	}
}

func (b *bridgeStateGetter) GetBridgeState(ctx context.Context, blockHash common.Hash) (*BridgeState, error) {
	eg, gCtx := errgroup.WithContext(ctx)

	var l1MessageHash *big.Int
	eg.Go(func() error {
		ret, err := callContract[bytes32](gCtx, b.nilClient, blockHash, b.contractAddr, b.abi, "l1MessageHash")
		if err != nil {
			return err
		}
		l1MessageHash = new(big.Int).SetBytes(ret[:])
		return nil
	})

	var l2ToL1Root *big.Int
	eg.Go(func() error {
		ret, err := callContract[bytes32](gCtx, b.nilClient, blockHash, b.contractAddr, b.abi, "getL2ToL1Root")
		if err != nil {
			return err
		}
		l2ToL1Root = new(big.Int).SetBytes(ret[:])
		return nil
	})

	var depositNonce *big.Int
	eg.Go(func() error {
		ret, err := callContract[*big.Int](gCtx, b.nilClient, blockHash, b.contractAddr, b.abi, "getLatestDepositNonce")
		if err != nil {
			return err
		}
		depositNonce = ret
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	ret := &BridgeState{
		L2toL1Root:    l2ToL1Root,
		DepositNonce:  depositNonce,
		L1MessageHash: l1MessageHash,
	}
	return ret, nil
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
