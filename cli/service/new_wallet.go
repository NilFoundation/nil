package service

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
)

const (
	ReceiptWaitFor  = 15 * time.Second
	ReceiptWaitTick = 200 * time.Millisecond
)

// TODO: Use a generic constant after adding it.
var gasPrice = *types.NewUint256(10)

func getFaucetAddress() types.Address {
	zeroStateConfig, err := execution.ParseZeroStateConfig(execution.DefaultZeroStateConfig)
	check.PanicIfErr(err)
	faucetAddress := zeroStateConfig.GetContractAddress("Faucet")
	check.PanicIfErr(err)
	return *faucetAddress
}

func (s *Service) WaitForReceipt(shardId types.ShardId, mshHash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return concurrent.WaitFor(context.Background(), ReceiptWaitFor, ReceiptWaitTick, func(ctx context.Context) (*jsonrpc.RPCReceipt, error) {
		return s.client.GetInMessageReceipt(shardId, mshHash)
	})
}

type MessageHashMismatchError struct {
	actual   common.Hash
	expected common.Hash
}

func (e MessageHashMismatchError) Error() string {
	return fmt.Sprintf("Unexpected sentmessage hash %s, expected %s", e.actual, e.expected)
}

func (s *Service) sendViaFaucet(to types.Address, value types.Uint256, privateKey *ecdsa.PrivateKey) error {
	gasLimit := *types.NewUint256(100_000)
	value.Add(&value.Int, types.NewUint256(0).Mul(&gasLimit.Int, &gasPrice.Int))
	sendMsgInternal := &types.InternalMessagePayload{
		To:       to,
		Value:    value,
		GasLimit: gasLimit,
		Kind:     types.ExecutionMessageKind,
	}
	sendMsgInternalData, err := sendMsgInternal.MarshalSSZ()
	if err != nil {
		return err
	}

	// Make external message to the Faucet
	faucetAbi, err := contracts.GetAbi("Faucet")
	check.PanicIfErr(err)
	calldata, err := faucetAbi.Pack("send", sendMsgInternalData)
	if err != nil {
		return err
	}

	from := getFaucetAddress()
	seqno, err := s.client.GetTransactionCount(from, "latest")
	if err != nil {
		return err
	}

	sendMsgExternal := &types.ExternalMessage{
		Seqno: seqno,
		To:    from,
		Data:  calldata,
	}
	err = sendMsgExternal.Sign(privateKey) // From should accept our signature
	if err != nil {
		return err
	}

	result, err := s.client.SendMessage(sendMsgExternal)
	if err != nil {
		return err
	}
	msgHash := sendMsgExternal.Hash()
	if msgHash != result {
		return MessageHashMismatchError{result, msgHash}
	}

	receipt, err := s.WaitForReceipt(to.ShardId(), sendMsgExternal.Hash())
	if err != nil {
		return errors.New("error during waiting for receipt")
	}

	if receipt == nil {
		return errors.New("receipt not received")
	}
	if !receipt.Success {
		return errors.New("send message processing failed")
	}
	return nil
}

func (s *Service) deployContractToPrepaidAddressViaExternalMessage(
	address types.Address,
	deployPayload types.DeployPayload,
	ownerPrivateKey *ecdsa.PrivateKey,
) (types.Address, error) {
	seqno, err := s.client.GetTransactionCount(address, "latest")
	if err != nil {
		return types.Address{}, err
	}

	deployMsgExternal := &types.ExternalMessage{
		Seqno: seqno,
		To:    address,
		Data:  deployPayload.Bytes(),
		Kind:  types.DeployMessageKind,
	}
	err = deployMsgExternal.Sign(ownerPrivateKey)
	if err != nil {
		return types.Address{}, err
	}

	result, err := s.client.SendMessage(deployMsgExternal)
	if err != nil {
		return types.EmptyAddress, err
	}

	msgHash := deployMsgExternal.Hash()
	if msgHash != result {
		return types.EmptyAddress, MessageHashMismatchError{result, msgHash}
	}

	res, err := s.WaitForReceipt(address.ShardId(), msgHash)
	if err != nil {
		return types.EmptyAddress, errors.New("error during waiting for receipt")
	}

	if !res.Success {
		return types.EmptyAddress, errors.New("deploy message processing failed")
	}
	return res.ContractAddress, nil
}

func (s *Service) CreateWallet(shardId types.ShardId, code types.Code, salt types.Uint256, ownerPrivateKey *ecdsa.PrivateKey) (types.Address, error) {
	deployPayload := types.BuildDeployPayload(code, common.Hash(salt.Bytes32()))
	walletAddress := types.CreateAddress(shardId, deployPayload)

	deployGasLimit := *types.NewUint256(500_000)
	value := types.NewUint256(0)
	value.Mul(&deployGasLimit.Int, &gasPrice.Int)
	err := s.sendViaFaucet(walletAddress, *value, ownerPrivateKey)
	if err != nil {
		return types.EmptyAddress, err
	}

	addr, err := s.deployContractToPrepaidAddressViaExternalMessage(walletAddress, deployPayload, ownerPrivateKey)
	if err != nil {
		return types.EmptyAddress, err
	}
	if addr != walletAddress {
		return types.EmptyAddress, errors.New("contract was deployed to unexpected address")
	}
	return addr, nil
}
