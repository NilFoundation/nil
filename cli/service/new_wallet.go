package service

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
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

// Almost a copy of assert.Eventually.
func waitFor(condition func() bool, waitFor time.Duration, tick time.Duration) bool {
	ch := make(chan bool, 1)

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for tick := ticker.C; ; {
		select {
		case <-timer.C:
			return false
		case <-tick:
			tick = nil
			go func() { ch <- condition() }()
		case v := <-ch:
			if v {
				return true
			}
			tick = ticker.C
		}
	}
}

func (s *Service) waitForReceipt(shardId types.ShardId, mshHash common.Hash) *jsonrpc.RPCReceipt {
	var receipt *jsonrpc.RPCReceipt
	var err error
	waitFor(func() bool {
		receipt, err = s.client.GetInMessageReceipt(shardId, mshHash)
		if err != nil {
			return false
		}
		return receipt.IsComplete()
	}, 15*time.Second, 200*time.Millisecond)

	if receipt == nil || receipt.MsgHash != mshHash {
		return nil
	}
	return receipt
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

	receipt := s.waitForReceipt(to.ShardId(), sendMsgExternal.Hash())
	if receipt == nil {
		return errors.New("Receipt not received")
	}
	if !receipt.Success {
		return errors.New("Send message processing failed")
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

	res := s.waitForReceipt(address.ShardId(), msgHash)
	if !res.Success {
		return types.EmptyAddress, errors.New("Deploy message processing failed")
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
		return types.EmptyAddress, errors.New("Contract was deployed to unexpected address")
	}
	return addr, nil
}
