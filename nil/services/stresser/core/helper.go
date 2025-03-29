package core

import (
	"context"
	"errors"
	"fmt"
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

func (h *Helper) DeployContract(name string, shardId types.ShardId) (*Contract, error) {
	h.logger.Debug().Msgf("Start deploying contract: %s on shard %d", name, shardId)

	code, err := contracts.GetCode(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get code for %s: %w", name, err)
	}

	payload := types.BuildDeployPayload(code, types.GenerateRandomHash())

	addr := types.CreateAddress(shardId, payload)

	topUpValue := types.GasToValue(10_000_000_000)

	topUpTries := 3
	for ; topUpTries != 0; topUpTries-- {
		topUpValue = topUpValue.Mul(topUpValue)
		if err = h.TopUp(addr, topUpValue); err == nil {
			break
		} else {
			h.logger.Warn().Err(err).Msgf("Failed to top up %s", addr.Hex())
		}
	}

	if topUpTries == 0 {
		return nil, fmt.Errorf("failed to top up %s: %w", addr.Hex(), err)
	}

	h.logger.Debug().Msgf("Top-up success: %s", addr.Hex())

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
	Value   *types.Value
}

func (h *Helper) Call(contract *Contract, method string, params *TxParams, args ...any) (*Transaction, error) {
	calldata, err := contract.PackCallData(method, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack call data: %w", err)
	}
	feePack := params.FeePack
	if feePack.FeeCredit.IsZero() {
		feePack = types.NewFeePackFromGas(1_000_000)
	}
	tx, err := h.Client.SendExternalTransaction(h.ctx, calldata, contract.Address, nil, feePack)
	if err != nil {
		return nil, fmt.Errorf("failed to send external transaction: %w", err)
	}

	txn := NewTransaction(tx)

	return txn, nil
}

func (h *Helper) TopUp(addr types.Address, value types.Value) error {
	if tx, err := h.faucet.TopUpViaFaucet(types.FaucetAddress, addr, value); err != nil {
		return fmt.Errorf("failed to top up via faucet: %w", err)
	} else {
		if receipt, err := h.WaitTx(tx); err != nil {
			return fmt.Errorf("failed to get receipt %s during top up: %w", tx, err)
		} else if !receipt.AllSuccess() {
			return fmt.Errorf("failed to top up via faucet: %s", receipt.Status)
		}
	}
	return nil
}

func (h *Helper) WaitTx(tx common.Hash) (*jsonrpc.RPCReceipt, error) {
	return common.WaitForValue(h.ctx, 30*time.Second, 500*time.Millisecond,
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
