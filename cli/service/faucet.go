package service

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	ReceiptWaitFor  = 15 * time.Second
	ReceiptWaitTick = 200 * time.Millisecond
)

var ErrWalletExists = errors.New("wallet already exists")

func collectFailedReceipts(dst []*jsonrpc.RPCReceipt, receipt *jsonrpc.RPCReceipt) []*jsonrpc.RPCReceipt {
	if !receipt.Success {
		dst = append(dst, receipt)
	}
	for _, r := range receipt.OutReceipts {
		dst = collectFailedReceipts(dst, r)
	}
	return dst
}

func (s *Service) WaitForReceipt(shardId types.ShardId, mshHash common.Hash) (*jsonrpc.RPCReceipt, error) {
	receipt, err := concurrent.WaitFor(context.Background(), ReceiptWaitFor, ReceiptWaitTick, func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
		receipt, err := s.client.GetInMessageReceipt(shardId, mshHash)
		if err != nil {
			return nil, err
		}
		if !receipt.IsComplete() {
			return nil, nil
		}
		return receipt, nil
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("Error during waiting for receipt")
		return nil, err
	}
	if receipt == nil {
		err := errors.New("successful receipt not received")
		s.logger.Error().Err(err).Send()
		return nil, err
	}

	failed := collectFailedReceipts(nil, receipt)

	if len(failed) > 0 {
		if !receipt.Success {
			s.logger.Error().Msgf("Message processing failed: %s", receipt.ErrorMessage)

			if len(receipt.OutReceipts) > 0 {
				s.logger.Error().Msg("Failed message has out messages. Report to the developers.")
			}
		} else {
			for _, r := range failed {
				if !r.Success {
					s.logger.Error().Msgf("Out message %s failed: %s", r.MsgHash, r.ErrorMessage)
				}
			}
		}

		receiptDataJSON, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to marshal unsuccessful receipt data to JSON")
			return nil, err
		}
		s.logger.Debug().Msgf("Full receipt:\n%s", receiptDataJSON)
	}
	return receipt, nil
}

type MessageHashMismatchError struct {
	actual   common.Hash
	expected common.Hash
}

func (e MessageHashMismatchError) Error() string {
	return fmt.Sprintf("Unexpected message hash %s, expected %s", e.actual, e.expected)
}

func (s *Service) TopUpViaFaucet(contractAddress types.Address, amount types.Value) error {
	msgHash, err := s.client.TopUpViaFaucet(contractAddress, amount)
	if err != nil {
		return err
	}

	_, err = s.WaitForReceipt(types.FaucetAddress.ShardId(), msgHash)
	if err != nil {
		return err
	}

	s.logger.Info().Msgf("Contract %s balance is topped up by %s", contractAddress, amount)
	return nil
}

func (s *Service) CreateWallet(shardId types.ShardId, salt *types.Uint256, balance types.Value, pubKey *ecdsa.PublicKey) (types.Address, error) {
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(pubKey))
	walletAddress := s.ContractAddress(shardId, *salt, walletCode)

	code, err := s.client.GetCode(walletAddress, "latest")
	if err != nil {
		return types.EmptyAddress, err
	}
	if len(code) > 0 {
		return types.EmptyAddress, ErrWalletExists
	}

	// NOTE: we deploy wallet code with ext message
	// in current implementation this costs 629_160
	err = s.TopUpViaFaucet(walletAddress, balance)
	if err != nil {
		return types.EmptyAddress, err
	}

	deployPayload := types.BuildDeployPayload(walletCode, common.Hash(salt.Bytes32()))
	msgHash, addr, err := s.DeployContractExternal(shardId, deployPayload)
	if err != nil {
		return types.EmptyAddress, err
	}
	check.PanicIfNotf(addr == walletAddress, "contract was deployed to unexpected address")
	res, err := s.WaitForReceipt(addr.ShardId(), msgHash)
	if err != nil {
		return types.EmptyAddress, errors.New("error during waiting for receipt")
	}
	if !res.IsComplete() {
		return types.EmptyAddress, errors.New("deploy message processing failed")
	}
	return addr, nil
}
