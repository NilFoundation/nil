package service

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	ReceiptWaitFor  = 15 * time.Second
	ReceiptWaitTick = 200 * time.Millisecond
)

var ErrWalletExists = errors.New("wallet already exists")

func (s *Service) WaitForReceipt(shardId types.ShardId, mshHash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return concurrent.WaitFor(context.Background(), ReceiptWaitFor, ReceiptWaitTick, func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
		receipt, err := s.client.GetInMessageReceipt(shardId, mshHash)
		if err != nil {
			return nil, err
		}
		if !receipt.IsComplete() {
			return nil, nil
		}
		return receipt, nil
	})
}

type MessageHashMismatchError struct {
	actual   common.Hash
	expected common.Hash
}

func (e MessageHashMismatchError) Error() string {
	return fmt.Sprintf("Unexpected message hash %s, expected %s", e.actual, e.expected)
}

func (s *Service) TopUpViaFaucet(contractAddress types.Address, amount *types.Uint256) error {
	msgHash, err := s.client.TopUpViaFaucet(contractAddress, amount)
	if err != nil {
		return err
	}

	receipt, err := s.WaitForReceipt(types.FaucetAddress.ShardId(), msgHash)
	if err != nil {
		return errors.New("error during waiting for receipt")
	}

	if receipt == nil {
		return errors.New("receipt not received")
	}

	if !receipt.Success {
		return errors.New("send message processing failed")
	}

	s.logger.Info().Msgf("Contract %s balance is topped up by %s", contractAddress, amount)
	return nil
}

func (s *Service) CreateWallet(shardId types.ShardId, salt types.Uint256, pubKey *ecdsa.PublicKey) (types.Address, error) {
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(pubKey))
	deployPayload := types.BuildDeployPayload(walletCode, common.Hash(salt.Bytes32()))
	walletAddress := types.CreateAddress(shardId, deployPayload)

	code, err := s.client.GetCode(walletAddress, "latest")
	if err != nil {
		return types.EmptyAddress, err
	}
	if len(code) > 0 {
		return types.EmptyAddress, ErrWalletExists
	}

	deployGasLimit := *types.NewUint256(1_000_000)
	value := types.NewUint256(0)
	value.Mul(&deployGasLimit.Int, execution.GasPrice)
	err = s.TopUpViaFaucet(walletAddress, value)
	if err != nil {
		return types.EmptyAddress, err
	}

	msgHash, addr, err := s.DeployContractExternal(shardId, deployPayload)
	if err != nil {
		return types.EmptyAddress, err
	}
	if addr != walletAddress {
		return types.EmptyAddress, errors.New("contract was deployed to unexpected address")
	}
	res, err := s.WaitForReceipt(addr.ShardId(), msgHash)
	if err != nil {
		return types.EmptyAddress, errors.New("error during waiting for receipt")
	}
	if !res.IsComplete() {
		return types.EmptyAddress, errors.New("deploy message processing failed")
	}
	return addr, nil
}
