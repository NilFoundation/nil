package cliservice

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	ReceiptWaitFor  = 20 * time.Second
	ReceiptWaitTick = 500 * time.Millisecond
)

var ErrSmartAccountExists = errors.New("smart account already exists")

func collectFailedReceipts(dst []*jsonrpc.RPCReceipt, receipt *jsonrpc.RPCReceipt) []*jsonrpc.RPCReceipt {
	if !receipt.Success {
		dst = append(dst, receipt)
	}
	for _, r := range receipt.OutReceipts {
		dst = collectFailedReceipts(dst, r)
	}
	return dst
}

func (s *Service) handleReceipt(txhash common.Hash, receipt *jsonrpc.RPCReceipt, err error) (*jsonrpc.RPCReceipt, error) {
	if err != nil {
		s.logger.Error().
			Err(err).
			Stringer(logging.FieldTransactionHash, txhash).
			Msg("Error during waiting for receipt")
		return nil, err
	}
	if receipt == nil {
		err := errors.New("successful receipt not received")
		s.logger.Error().Msg("Successful receipt not received")
		return nil, err
	}

	failed := collectFailedReceipts(nil, receipt)

	if len(failed) > 0 {
		if !receipt.Success {
			s.logger.Error().Str(logging.FieldError, receipt.ErrorMessage).Msg("Failed transaction processing.")

			if len(receipt.OutReceipts) > 0 {
				s.logger.Error().Msg("Failed transaction has outgoing transactions. Report to the developers.")
			}
		} else {
			s.logger.Info().Msg("Failed outgoing transactions:")
			for _, r := range failed {
				if !r.Success {
					s.logger.Error().
						Str("status", r.Status).
						Str(logging.FieldError, r.ErrorMessage).
						Stringer(logging.FieldTransactionHash, r.TxnHash).
						Msg("Failed transaction processing")
				}
			}
		}

		receiptDataJSON, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to marshal unsuccessful receipt data to JSON")
			return nil, err
		}

		debug := s.logger.Debug()
		if debug == nil {
			s.logger.Info().Msg("To view full receipts, run with debug log level or use `nil receipt`.")
		} else {
			debug.RawJSON(logging.FieldFullTransaction, receiptDataJSON).Msg("Full transaction receipt")
		}
	}
	return receipt, nil
}

func (s *Service) waitForReceiptCommon(txnHash common.Hash, check func(receipt *jsonrpc.RPCReceipt) bool) (*jsonrpc.RPCReceipt, error) {
	receipt, err := concurrent.WaitFor(s.ctx, ReceiptWaitFor, ReceiptWaitTick, func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
		receipt, err := s.client.GetInTransactionReceipt(ctx, txnHash)
		if err != nil {
			return nil, err
		}
		if !check(receipt) {
			return nil, nil
		}
		return receipt, nil
	})
	return s.handleReceipt(txnHash, receipt, err)
}

func (s *Service) WaitForReceipt(txnHash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return s.waitForReceiptCommon(txnHash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsComplete()
	})
}

func (s *Service) WaitForReceiptCommitted(txnHash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return s.waitForReceiptCommon(txnHash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsCommitted()
	})
}

type TransactionHashMismatchError struct {
	actual   common.Hash
	expected common.Hash
}

func (e TransactionHashMismatchError) Error() string {
	return fmt.Sprintf("Unexpected transaction hash %s, expected %s", e.actual, e.expected)
}

func (s *Service) TopUpViaFaucet(faucetAddress, contractAddressTo types.Address, amount types.Value) error {
	if s.faucetClient == nil {
		return errors.New("faucet client is not set")
	}

	var r *jsonrpc.RPCReceipt
	for range 5 {
		txnHash, err := s.faucetClient.TopUpViaFaucet(faucetAddress, contractAddressTo, amount)
		if err != nil {
			return err
		}

		r, err = s.WaitForReceipt(txnHash)
		if err != nil {
			return err
		}

		if r.AllSuccess() {
			s.logger.Info().Msgf("Contract %s balance is topped up by %s on behalf of %s", contractAddressTo, amount, faucetAddress)
			return nil
		}
	}

	return fmt.Errorf("failed to top up contract %s: %s", contractAddressTo, r.ErrorMessage)
}

func (s *Service) CreateSmartAccount(
	shardId types.ShardId,
	salt *types.Uint256,
	balance types.Value,
	fee types.FeePack,
	pubKey *ecdsa.PublicKey,
) (types.Address, error) {
	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(crypto.CompressPubkey(pubKey))
	smartAccountAddress := s.ContractAddress(shardId, *salt, smartAccountCode)

	code, err := s.client.GetCode(s.ctx, smartAccountAddress, "latest")
	if err != nil {
		return types.EmptyAddress, err
	}
	if len(code) > 0 {
		return types.EmptyAddress, fmt.Errorf("%w: %s", ErrSmartAccountExists, smartAccountAddress)
	}

	// NOTE: we deploy smart account code with ext transaction
	// in current implementation this costs 629_160
	err = s.TopUpViaFaucet(types.FaucetAddress, smartAccountAddress, balance)
	if err != nil {
		return types.EmptyAddress, err
	}

	deployPayload := types.BuildDeployPayload(smartAccountCode, common.Hash(salt.Bytes32()))
	txnHash, addr, err := s.DeployContractExternal(shardId, deployPayload, fee)
	if err != nil {
		return types.EmptyAddress, err
	}
	check.PanicIfNotf(addr == smartAccountAddress, "contract was deployed to unexpected address")
	res, err := s.WaitForReceipt(txnHash)
	if err != nil {
		return types.EmptyAddress, errors.New("error during waiting for receipt")
	}
	if !res.IsComplete() {
		return types.EmptyAddress, errors.New("deploy transaction processing failed")
	}
	if !res.AllSuccess() {
		return types.EmptyAddress, fmt.Errorf("deploy transaction processing failed: %s", res.ErrorMessage)
	}
	return addr, nil
}
