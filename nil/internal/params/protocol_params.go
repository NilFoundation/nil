// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

const (
	// Times ceil(log256(exponent)) for the EXP instruction.
	ExpByteGas uint64 = 10
	// Multiplied by the number of 32-byte words that are copied (round up) for any *COPY operation and added.
	SloadGas uint64 = 50
	// Paid for CALL when the value transfer is non-zero.
	CallValueTransferGas uint64 = 9000
	// Paid for CALL when the destination address didn't exist prior.
	CallNewAccountGas uint64 = 25000
	// Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions.
	TxGas uint64 = 21000
	// Per transaction that creates a contract. NOTE: Not payable on data of calls between transactions.
	TxGasContractCreation uint64 = 53000
	// Per byte of data attached to a transaction that equals zero.
	// NOTE: Not payable on data of calls between transactions.
	TxDataZeroGas uint64 = 4
	// Divisor for the quadratic particle of the memory cost equation.
	QuadCoeffDiv uint64 = 512
	// Per byte in a LOG* operation's data.
	LogDataGas uint64 = 8
	// Free gas given at beginning of call.
	CallStipend uint64 = 2300

	// Once per KECCAK256 operation.
	Keccak256Gas uint64 = 30
	// Once per word of the KECCAK256 operation's data.
	Keccak256WordGas uint64 = 6
	// Once per word of the init code when creating a contract.
	InitCodeWordGas uint64 = 2

	// Once per SSTORE operation.
	SstoreSetGas uint64 = 20000
	// Once per SSTORE operation if the zeroness changes from zero.
	SstoreResetGas uint64 = 5000
	// Once per SSTORE operation if the zeroness doesn't change.
	SstoreClearGas uint64 = 5000
	// Once per SSTORE operation if the zeroness changes to zero.
	SstoreRefundGas uint64 = 15000

	// Once per SSTORE operation if the value doesn't change.
	NetSstoreNoopGas uint64 = 200
	// Once per SSTORE operation from clean zero.
	NetSstoreInitGas uint64 = 20000
	// Once per SSTORE operation from clean non-zero.
	NetSstoreCleanGas uint64 = 5000
	// Once per SSTORE operation from dirty.
	NetSstoreDirtyGas uint64 = 200

	// Once per SSTORE operation for clearing an originally existing storage slot
	NetSstoreClearRefund uint64 = 15000
	// Once per SSTORE operation for resetting to the original non-zero value
	NetSstoreResetClearRefund uint64 = 19800
	// Once per SSTORE operation for resetting to the original zero value
	NetSstoreResetRefund uint64 = 4800

	// Minimum gas required to be present for an SSTORE call, not consumed
	SstoreSentryGasEIP2200 uint64 = 2300
	// Once per SSTORE operation from clean zero to non-zero
	SstoreSetGasEIP2200 uint64 = 20000
	// Once per SSTORE operation from clean non-zero to something else
	SstoreResetGasEIP2200 uint64 = 5000
	// Once per SSTORE operation for clearing an originally existing storage slot
	SstoreClearsScheduleRefundEIP2200 uint64 = 15000

	// COLD_ACCOUNT_ACCESS_COST
	ColdAccountAccessCostEIP2929 = uint64(2600)
	// COLD_SLOAD_COST
	ColdSloadCostEIP2929 = uint64(2100)
	// WARM_STORAGE_READ_COST
	WarmStorageReadCostEIP2929 = uint64(100)

	// In EIP-2200: SstoreResetGas was 5000.
	// In EIP-2929: SstoreResetGas was changed to '5000 - COLD_SLOAD_COST'.
	// In EIP-3529: SSTORE_CLEARS_SCHEDULE is defined as SSTORE_RESET_GAS + ACCESS_LIST_STORAGE_KEY_COST.
	// Which becomes: 5000 - 2100 + 1900 = 4800.
	SstoreClearsScheduleRefundEIP3529 uint64 = SstoreResetGasEIP2200 - ColdSloadCostEIP2929 + TxAccessListStorageKeyGas

	// Once per JUMPDEST operation.
	JumpdestGas uint64 = 1
	// Duration between proof-of-work epochs.
	EpochDuration uint64 = 30000

	CreateDataGas uint64 = 200
	// Maximum depth of call/create stack.
	CallCreateDepth uint64 = 1024
	// Once per EXP instruction
	ExpGas uint64 = 10
	// Per LOG* operation.
	LogGas  uint64 = 375
	CopyGas uint64 = 3
	// Maximum size of VM stack allowed.
	StackLimit uint64 = 1024
	// Once per operation, for a selection of them.
	TierStepGas uint64 = 0
	// Multiplied by the * of the LOG*, per LOG transaction.
	// e.g. LOG0 incurs 0 * c_txLogTopicGas, LOG4 incurs 4 * c_txLogTopicGas.
	LogTopicGas uint64 = 375
	// Once per CREATE operation & contract-creation transaction.
	CreateGas uint64 = 32000
	// Once per CREATE2 operation
	Create2Gas uint64 = 32000
	// Refunded following a selfdestruct operation.
	SelfdestructRefundGas uint64 = 24000
	// Times the address of the (highest referenced byte in memory + 1).
	// NOTE: referencing happens on read, write and in instructions such as RETURN and CALL.
	MemoryGas uint64 = 3

	// Per byte of data attached to a transaction that is not equal to zero.
	// NOTE: Not payable on data of calls between transactions.
	TxDataNonZeroGasFrontier uint64 = 68
	// Per byte of non-zero data attached to a transaction after EIP 2028 (part in Istanbul)
	TxDataNonZeroGasEIP2028 uint64 = 16
	// Per address specified in EIP 2930 access list
	TxAccessListAddressGas uint64 = 2400
	// Per storage key specified in EIP 2930 access list
	TxAccessListStorageKeyGas uint64 = 1900

	// These have been changed during the course of the chain.
	// Once per CALL operation & transaction call.
	CallGasFrontier uint64 = 40
	// Static portion of gas for CALL-derivatives after EIP 150 (Tangerine)
	CallGasEIP150 uint64 = 700
	// The cost of a BALANCE operation
	BalanceGasFrontier uint64 = 20
	// The cost of a BALANCE operation after Tangerine
	BalanceGasEIP150 uint64 = 400
	// The cost of a BALANCE operation after EIP 1884 (part of Istanbul)
	BalanceGasEIP1884 uint64 = 700
	// Cost of EXTCODESIZE before EIP 150 (Tangerine)
	ExtcodeSizeGasFrontier uint64 = 20
	// Cost of EXTCODESIZE after EIP 150 (Tangerine)
	ExtcodeSizeGasEIP150 uint64 = 700
	SloadGasFrontier     uint64 = 50
	SloadGasEIP150       uint64 = 200
	// Cost of SLOAD after EIP 1884 (part of Istanbul)
	SloadGasEIP1884 uint64 = 800
	// Cost of SLOAD after EIP 2200 (part of Istanbul)
	SloadGasEIP2200 uint64 = 800
	// Cost of EXTCODEHASH (introduced in Constantinople)
	ExtcodeHashGasConstantinople uint64 = 400
	// Cost of EXTCODEHASH after EIP 1884 (part in Istanbul)
	ExtcodeHashGasEIP1884 uint64 = 700
	// Cost of SELFDESTRUCT post EIP 150 (Tangerine)
	SelfdestructGasEIP150 uint64 = 5000

	// EXP has a dynamic portion depending on the size of the exponent.
	// was set to 10 in Frontier
	ExpByteFrontier uint64 = 10
	// was raised to 50 during Eip158 (Spurious Dragon)
	ExpByteEIP158 uint64 = 50

	// Extcodecopy has a dynamic AND a static cost. This represents only the
	// static portion of the gas. It was changed during EIP 150 (Tangerine).
	ExtcodeCopyBaseFrontier uint64 = 20
	ExtcodeCopyBaseEIP150   uint64 = 700

	// CreateBySelfdestructGas is used when the refunded account is one that does
	// not exist. This logic is similar to call.
	// Introduced in Tangerine Whistle (Eip 150)
	CreateBySelfdestructGas uint64 = 25000

	// Bounds the amount the base fee can change between blocks.
	DefaultBaseFeeChangeDenominator = 8
	// Bounds the maximum gas limit an EIP-1559 block may have.
	DefaultElasticityMultiplier = 2
	// Initial base fee for EIP-1559 blocks.
	InitialBaseFee = 1000000000

	// Maximum bytecode to permit for a contract
	MaxCodeSize = 24576
	// Maximum initcode to permit in a creation transaction and create instructions
	MaxInitCodeSize = 2 * MaxCodeSize

	// Precompiled contract gas prices

	// Elliptic curve sender recovery gas price
	EcrecoverGas uint64 = 3000
	// Base price for a SHA256 operation
	Sha256BaseGas uint64 = 60
	// Per-word price for a SHA256 operation
	Sha256PerWordGas uint64 = 12
	// Base price for a RIPEMD160 operation
	Ripemd160BaseGas uint64 = 600
	// Per-word price for a RIPEMD160 operation
	Ripemd160PerWordGas uint64 = 120
	// Base price for a data copy operation
	IdentityBaseGas uint64 = 15
	// Per-work price for a data copy operation
	IdentityPerWordGas uint64 = 3

	// Byzantium gas needed for an elliptic curve addition
	Bn256AddGasByzantium uint64 = 500
	// Gas needed for an elliptic curve addition
	Bn256AddGasIstanbul uint64 = 150
	// Byzantium gas needed for an elliptic curve scalar multiplication
	Bn256ScalarMulGasByzantium uint64 = 40000
	// Gas needed for an elliptic curve scalar multiplication
	Bn256ScalarMulGasIstanbul uint64 = 6000
	// Byzantium base price for an elliptic curve pairing check
	Bn256PairingBaseGasByzantium uint64 = 100000
	// Base price for an elliptic curve pairing check
	Bn256PairingBaseGasIstanbul uint64 = 45000
	// Byzantium per-point price for an elliptic curve pairing check
	Bn256PairingPerPointGasByzantium uint64 = 80000
	// Per-point price for an elliptic curve pairing check
	Bn256PairingPerPointGasIstanbul uint64 = 34000

	// Price for BLS12-381 elliptic curve G1 point addition
	Bls12381G1AddGas uint64 = 500
	// Price for BLS12-381 elliptic curve G1 point scalar multiplication
	Bls12381G1MulGas uint64 = 12000
	// Price for BLS12-381 elliptic curve G2 point addition
	Bls12381G2AddGas uint64 = 800
	// Price for BLS12-381 elliptic curve G2 point scalar multiplication
	Bls12381G2MulGas uint64 = 45000
	// Base gas price for BLS12-381 elliptic curve pairing check
	Bls12381PairingBaseGas uint64 = 65000
	// Per-point pair gas price for BLS12-381 elliptic curve pairing check
	Bls12381PairingPerPairGas uint64 = 43000
	// Gas price for BLS12-381 mapping field element to G1 operation
	Bls12381MapG1Gas uint64 = 5500
	// Gas price for BLS12-381 mapping field element to G2 operation
	Bls12381MapG2Gas uint64 = 75000

	// Size in bytes of a field element
	BlobTxBytesPerFieldElement = 32
	// Number of field elements stored in a single data blob
	BlobTxFieldElementsPerBlob = 4096
	// Gas consumption of a single data blob (== blob byte size)
	BlobTxBlobGasPerBlob = 1 << 17
	// Minimum gas price for data blobs
	BlobTxMinBlobGasprice = 1
	// Controls the maximum rate of change for blob gas price
	BlobTxBlobGaspriceUpdateFraction = 3338477
	// Gas price for the point evaluation precompile.
	BlobTxPointEvaluationPrecompileGas = 50000

	// Target consumable blob gas for data blobs per block (for 1559-like pricing)
	BlobTxTargetBlobGasPerBlock = 3 * BlobTxBlobGasPerBlob
	// Maximum consumable blob gas for data blobs per block
	MaxBlobGasPerBlock = 6 * BlobTxBlobGasPerBlob
)

// Gas discount table for BLS12-381 G1 and G2 multi exponentiation operations
var Bls12381MultiExpDiscountTable = [128]uint64{
	1200, 888, 764, 641, 594, 547, 500, 453, 438, 423, 408, 394, 379, 364, 349, 334, 330, 326, 322, 318, 314, 310, 306,
	302, 298, 294, 289, 285, 281, 277, 273, 269, 268, 266, 265, 263, 262, 260, 259, 257, 256, 254, 253, 251, 250, 248,
	247, 245, 244, 242, 241, 239, 238, 236, 235, 233, 232, 231, 229, 228, 226, 225, 223, 222, 221, 220, 219, 219, 218,
	217, 216, 216, 215, 214, 213, 213, 212, 211, 211, 210, 209, 208, 208, 207, 206, 205, 205, 204, 203, 202, 202, 201,
	200, 199, 199, 198, 197, 196, 196, 195, 194, 193, 193, 192, 191, 191, 190, 189, 188, 188, 187, 186, 185, 185, 184,
	183, 182, 182, 181, 180, 179, 179, 178, 177, 176, 176, 175, 174,
}
