package rpctest

import (
	"encoding/binary"
	"slices"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
)

// this file contains list of hardcoded contracts
// which are temporary used in the prototype code

// TODO: remove this after adding solidity compiler

func numToBytes(n int) []byte {
	lenAsBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenAsBytes, uint32(n))
	return lenAsBytes
}

func initHardcodedCallData() ([]byte, int) {
	var res []byte
	msgData, _ := (&types.Message{
		Seqno: 123,
		From:  common.BytesToAddress([]byte("from-address")),
		To:    common.BytesToAddress([]byte("to-address")),
	}).MarshalSSZ()

	realLen := len(msgData)
	for len(msgData)%32 != 0 {
		msgData = append(msgData, 0)
	}

	for i := 0; i*32 < len(msgData); i++ {
		res = append(res, byte(vm.PUSH32))
		res = append(res, msgData[i*32:(i+1)*32]...)
		res = append(res, byte(vm.PUSH1), byte(i*32))
		res = append(res, byte(vm.MSTORE))
	}
	return res, realLen
}

var sendMessageCalldata, realLen = initHardcodedCallData()

var hardcodedIncAndAddMsgContract = slices.Concat(
	[]byte{
		// just increment the value stored at `storage[0]` cell
		byte(vm.PUSH1), 0x0,
		byte(vm.SLOAD),
		byte(vm.PUSH1), 0x1,
		byte(vm.ADD),
		byte(vm.PUSH1), 0x0,
		byte(vm.SSTORE),
	},
	sendMessageCalldata,
	[]byte{
		// call add-message precompiled contract
		byte(vm.PUSH1), 0x0,
		byte(vm.PUSH1), 0x0,
	},
	[]byte{byte(vm.PUSH4)}, numToBytes(realLen),
	[]byte{
		byte(vm.PUSH1), 0x0,
		byte(vm.PUSH1), 0x0,
		byte(vm.PUSH1), 0x6,
		byte(vm.PUSH1), 0x0,
		byte(vm.CALL),
	},
)

var HardcodedContract = slices.Concat(
	// // here we just return N-byte value which is actually a contract code
	// // now contractCreator is called from shardchain for new contract creation
	// []byte{byte(vm.PUSH4)}, numToBytes(len(hardcodedIncAndAddMsgContract)),
	// []byte{
	// 	byte(vm.PUSH1), 0x10, /*= len of this preamble */
	// 	byte(vm.PUSH0),
	// 	byte(vm.CODECOPY),
	// },
	// []byte{byte(vm.PUSH4)}, numToBytes(len(hardcodedIncAndAddMsgContract)),
	// []byte{
	// 	byte(vm.PUSH0),
	// 	byte(vm.RETURN),
	// },
	hardcodedIncAndAddMsgContract,
)
