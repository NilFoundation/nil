package feeupdater

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/logging"
	corebind "github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/v2"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type NilGasPriceOracleContract interface {
	SetOracleFee(ctx context.Context, params feeParams) error
}

type EthClient interface {
	bind.ContractBackend
	bind.DeployBackend
	ChainID(ctx context.Context) (*big.Int, error)
}

type ContractWrapperConfig struct {
	ContractAddress string `yaml:"feeupdaterL1ContractAddress"`
	PrivateKey      string `yaml:"feeupdaterL1PrivateKey"`
}

func (cfg *ContractWrapperConfig) Validate() error {
	if cfg.ContractAddress == "" {
		return errors.New("IFeeStorage contract address is empty")
	}
	if cfg.PrivateKey == "" {
		return errors.New("private key for IFeeStorage L1 contract is empty")
	}
	return nil
}

type wrapper struct {
	client     EthClient
	impl       *Feeupdater
	privateKey *ecdsa.PrivateKey
	chainID    *big.Int
	logger     logging.Logger
}

func NewWrapper(
	ctx context.Context,
	cfg *ContractWrapperConfig,
	client EthClient,
) (*wrapper, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid fee updater L1 config: %w", err)
	}

	addr := ethcommon.HexToAddress(cfg.ContractAddress)

	privateKeyECDSA, err := crypto.HexToECDSA(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("converting private key hex to ECDSA: %w", err)
	}

	impl, err := NewFeeupdater(addr, client)
	if err != nil {
		return nil, err
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting chain ID: %w", err)
	}

	return &wrapper{
		impl:       impl,
		privateKey: privateKeyECDSA,
		chainID:    chainID,
	}, nil
}

func (w *wrapper) SetOracleFee(ctx context.Context, params feeParams) error {
	bindOpts, err := corebind.NewKeyedTransactorWithChainID(w.privateKey, w.chainID)
	if err != nil {
		return fmt.Errorf("creating new keyed transactor: %w", err)
	}

	tx, err := w.impl.SetOracleFee(
		bindOpts,
		params.maxFeePerGas,
		params.maxPriorityFeePerGas,
	)
	if err != nil {
		return fmt.Errorf("failed to submit fee update transaction: %w", err)
	}

	receipt, err := corebind.WaitMined(ctx, w.client, tx)

	switch {
	case err != nil:
		return fmt.Errorf("failed to fetch transaction receipt: %w", err)
	case receipt == nil:
		return fmt.Errorf("transaction receipt is not received: %w", err)
	case receipt.Status != ethtypes.ReceiptStatusSuccessful:
		w.logReceiptDetails(receipt)
		return fmt.Errorf("transaction failed: %s", receipt.Logs[0].Data)
	default:
		w.logReceiptDetails(receipt)
		w.logger.Info().
			Hex("txHash", tx.Hash().Bytes()).
			Msg("setOracleFee transaction sent successfully")
		return nil
	}
}

func (r *wrapper) logReceiptDetails(receipt *ethtypes.Receipt) {
	r.logger.Info().
		Uint8("type", receipt.Type).
		Uint64("status", receipt.Status).
		Uint64("cumulativeGasUsed", receipt.CumulativeGasUsed).
		Hex("txHash", receipt.TxHash.Bytes()).
		Str("contractAddress", receipt.ContractAddress.Hex()).
		Uint64("gasUsed", receipt.GasUsed).
		Str("effectiveGasPrice", receipt.EffectiveGasPrice.String()).
		Hex("blockHash", receipt.BlockHash.Bytes()).
		Str("blockNumber", receipt.BlockNumber.String()).
		Uint("transactionIndex", receipt.TransactionIndex).
		Msg("transaction receipt received")
}
