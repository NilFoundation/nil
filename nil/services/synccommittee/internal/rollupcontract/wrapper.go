package rollupcontract

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type Wrapper struct {
	rollupContract *Rollupcontract
	requestTimeout time.Duration
	privateKey     *ecdsa.PrivateKey
	chainID        *big.Int
}

func NewWrapper(ctx context.Context, contractAddress string, privateKey string, ethClient EthClient, requestTimeout time.Duration) (*Wrapper, error) {
	rollupContract, err := NewRollupcontract(ethcommon.HexToAddress(contractAddress), ethClient)
	if err != nil {
		return nil, fmt.Errorf("can't create rollup contract instance: %w", err)
	}

	privateKeyECDSA, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("converting private key hex to ECDSA: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chain ID: %w", err)
	}

	return &Wrapper{
		rollupContract: rollupContract,
		requestTimeout: requestTimeout,
		privateKey:     privateKeyECDSA,
		chainID:        chainID,
	}, nil
}

func (r *Wrapper) ProofBatch(ctx context.Context, prevStateRoot common.Hash, newStateRoot common.Hash, proof []byte, batchIndexInBlobStorage *big.Int) (*ethtypes.Transaction, error) {
	transactOpts, cancel, err := r.getEthTransactOpts(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return r.rollupContract.ProofBatch(transactOpts, prevStateRoot, newStateRoot, proof, batchIndexInBlobStorage)
}

func (r *Wrapper) StateRoots(ctx context.Context, finalizedBatchIndex *big.Int) ([32]byte, error) {
	callOpts, cancel := r.getEthCallOpts(ctx)
	defer cancel()
	return r.rollupContract.StateRoots(callOpts, finalizedBatchIndex)
}

func (r *Wrapper) FinalizedBatchIndex(ctx context.Context) (*big.Int, error) {
	callOpts, cancel := r.getEthCallOpts(ctx)
	defer cancel()
	return r.rollupContract.FinalizedBatchIndex(callOpts)
}

func (r *Wrapper) getEthCallOpts(ctx context.Context) (*bind.CallOpts, context.CancelFunc) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.requestTimeout)
	return &bind.CallOpts{Context: ctxWithTimeout}, cancel
}

func (r *Wrapper) getEthTransactOpts(ctx context.Context) (*bind.TransactOpts, context.CancelFunc, error) {
	keyedTransactor, err := bind.NewKeyedTransactorWithChainID(r.privateKey, r.chainID)
	if err != nil {
		return nil, nil, fmt.Errorf("creating keyed transactor with chain ID: %w", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.requestTimeout)
	keyedTransactor.Context = ctxWithTimeout
	return keyedTransactor, cancel, nil
}
