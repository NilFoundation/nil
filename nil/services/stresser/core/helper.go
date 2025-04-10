package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/rs/zerolog"
)

type Helper struct {
	ctx    context.Context
	Client client.Client
	faucet *faucet.Client
	logger logging.Logger
}

func NewHelper(ctx context.Context, endpoint string) (*Helper, error) {
	c := &Helper{ctx: ctx}
	rpcLogger := logging.NewLogger("rpc").Level(zerolog.DebugLevel)
	c.Client = rpc.NewClient(endpoint, rpcLogger)
	if c.Client == nil {
		return nil, errors.New("failed to create rpc client")
	}
	c.faucet = faucet.NewClient(endpoint)
	if c.faucet == nil {
		return nil, errors.New("failed to create faucet client")
	}

	c.logger = logging.NewLogger("client")

	return c, nil
}

func (h *Helper) WaitClusterReady(numShards int) error {
	return common.WaitFor(h.ctx, time.Second*30, time.Second*2,
		func(ctx context.Context) bool {
			list, err := h.Client.GetShardIdList(ctx)
			return err == nil && len(list) == (numShards-1)
		})
}

func (h *Helper) DeployStressers(shardId types.ShardId, num int) ([]*Contract, error) {
	code, err := contracts.GetCode("tests/StresserFactory")
	if err != nil {
		return nil, fmt.Errorf("failed to get code for StresserFactory: %w", err)
	}
	payload := types.BuildDeployPayload(code, types.GenerateRandomHash())

	addr := types.CreateAddress(shardId, payload)

	topUpValue := types.GasToValue(100_000_000_000_000)
	balancePerStresser := topUpValue.Sub(types.GasToValue(10_000_000_000)).Div(types.NewValueFromUint64(uint64(num)))
	h.logger.Info().
		Stringer("topUpValue", topUpValue).
		Stringer("balancePerStresser", balancePerStresser).
		Int("num", num).
		Int("shard", int(shardId)).
		Msg("Start deploying stresses")

	topUpTries := 3
	for ; topUpTries != 0; topUpTries-- {
		if err = h.TopUp(addr, topUpValue); err != nil {
			h.logger.Warn().Err(err).Msgf("Failed to top up %s", addr.Hex())
		}
	}
	if topUpTries == 0 {
		return nil, fmt.Errorf("failed to top up %s: %w", addr.Hex(), err)
	}

	txHash, addr, err := h.Client.DeployExternal(h.ctx, shardId, payload, types.NewFeePackFromGas(100_000_000))
	if err != nil {
		return nil, fmt.Errorf("failed to deploy StresserFactory at %s: %w", addr, err)
	}
	receipt, err := common.WaitForValue(h.ctx, 20*time.Second, 500*time.Millisecond,
		func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
			return h.Client.GetInTransactionReceipt(ctx, txHash)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %w", err)
	}
	if !receipt.Success {
		return nil, fmt.Errorf("failed to deploy contract at %s: %s", addr, receipt.Status)
	}
	balance, _ := h.Client.GetBalance(h.ctx, addr, "latest")
	h.logger.Debug().Stringer("balance", balance).Msgf("Factory deployed on shard %d", shardId)

	factory, err := NewContract("tests/StresserFactory", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create factory contract: %w", err)
	}
	txparams := &TxParams{FeePack: types.NewFeePackFromGas(100_000_000)}
	receipt, err = h.CallAndWait(factory, "deployContracts", txparams, big.NewInt(int64(num)),
		balancePerStresser.ToBig())
	if err != nil {
		return nil, fmt.Errorf("failed to deploy stresses: %w", err)
	}
	if len(receipt.Logs) != 1 {
		return nil, fmt.Errorf("unexpected number of logs: %d(expected 1)", len(receipt.Logs))
	}
	unpacked, err := factory.Abi.Unpack("deployed", receipt.Logs[0].Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack Deployed event: %w", err)
	}
	if len(unpacked) != 1 {
		return nil, fmt.Errorf("unexpected number of arguments in `deployed` event: %d(expected 1)", len(unpacked))
	}
	addresses, ok := unpacked[0].([]types.Address)
	if !ok {
		return nil, errors.New("unexpected type of `deployed` event")
	}
	if len(addresses) != num {
		return nil, fmt.Errorf("unexpected number of deployed contracts: %d(expected %d)", len(addresses), num)
	}
	res := make([]*Contract, num)
	for i, addr := range addresses {
		res[i], err = NewContract("tests/Stresser", addr)
		if err != nil {
			return nil, fmt.Errorf("failed to create stresser contract: %w", err)
		}
	}

	return res, nil
}

func (h *Helper) DeployContract(name string, shardId types.ShardId) (*Contract, error) {
	h.logger.Debug().Msgf("Start deploying contract: %s on shard %d", name, shardId)

	code, err := contracts.GetCode(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get code for %s: %w", name, err)
	}

	payload := types.BuildDeployPayload(code, types.GenerateRandomHash())

	addr := types.CreateAddress(shardId, payload)

	topUpValue := types.GasToValue(100_000_000_000)

	topUpTries := 3
	for ; topUpTries != 0; topUpTries-- {
		if err = h.TopUp(addr, topUpValue); err == nil {
			break
		}
		h.logger.Warn().Err(err).Msgf("Failed to top up %x", addr)
	}

	if topUpTries == 0 {
		return nil, fmt.Errorf("failed to top up %x: %w", addr, err)
	}

	h.logger.Debug().Msgf("Top-up success: %x", addr)

	tx, addr, err := h.Client.DeployExternal(h.ctx, shardId, payload, types.NewFeePackFromGas(100_000_000))
	if err != nil {
		return nil, fmt.Errorf("failed to deploy contract at %s: %w", addr, err)
	}
	receipt, err := common.WaitForValue(h.ctx, 30*time.Second, 500*time.Millisecond,
		func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
			return h.Client.GetInTransactionReceipt(ctx, tx)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %w", err)
	}
	if !receipt.Success {
		return nil, fmt.Errorf("failed to deploy contract at %s: %s", addr, receipt.Status)
	}

	balance, err := h.Client.GetBalance(h.ctx, addr, "latest")
	if err != nil {
		h.logger.Error().Err(err).Stringer("addr", addr).Msg("Failed to get balance")
	}

	h.logger.Info().Msgf("Contract deployed at %x, balance: %s", addr, balance)

	return NewContract(name, addr)
}

type TxParams struct {
	FeePack types.FeePack
	Value   types.Value
}

func (h *Helper) Call(contract *Contract, method string, params *TxParams, args ...any) (common.Hash, error) {
	calldata, err := contract.PackCallData(method, args...)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to pack call data: %w", err)
	}
	feePack := params.FeePack
	if feePack.FeeCredit.IsZero() {
		feePack = types.NewFeePackFromGas(1_000_000)
	}
	txHash, err := h.Client.SendExternalTransaction(h.ctx, calldata, contract.Address, nil, feePack)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to send external transaction: %w", err)
	}
	return txHash, nil
}

func (h *Helper) CallAndWait(
	contract *Contract,
	method string,
	params *TxParams,
	args ...any,
) (*jsonrpc.RPCReceipt, error) {
	txHash, err := h.Call(contract, method, params, args...)
	if err != nil {
		return nil, err
	}
	receipt, err := common.WaitForValue(h.ctx, 300*time.Second, 500*time.Millisecond,
		func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
			return h.Client.GetInTransactionReceipt(ctx, txHash)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %w", err)
	}
	if !receipt.Success {
		return nil, fmt.Errorf("failed to call %s: %s", method, receipt.Status)
	}
	return receipt, nil
}

func (h *Helper) TopUp(addr types.Address, value types.Value) error {
	tx, err := h.faucet.TopUpViaFaucet(types.FaucetAddress, addr, value)
	if err != nil {
		return fmt.Errorf("failed to top up via faucet: %w", err)
	}
	if receipt, err := h.WaitTx(tx); err != nil {
		return fmt.Errorf("failed to get receipt %s during top up: %w", tx, err)
	} else if !receipt.AllSuccess() {
		return fmt.Errorf("failed to top up via faucet: %s", receipt.Status)
	}
	return nil
}

func (h *Helper) WaitTx(tx common.Hash) (*jsonrpc.RPCReceipt, error) {
	return common.WaitForValue(h.ctx, 10*time.Second, 1000*time.Millisecond,
		func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
			receipt, err := h.Client.GetInTransactionReceipt(ctx, tx)
			if err != nil {
				return nil, err
			}
			if !receipt.IsComplete() {
				return nil, nil
			}
			return receipt, nil
		})
}
