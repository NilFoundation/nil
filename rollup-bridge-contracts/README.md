# Nil L1-Contracts

This smart-contracts module contains contracts required for communication with L1.
Solidity smart contracts in the repository are used for proof verification and state root updates of L2 on the L1 chain

1. NilRollup
2. NilVerifier

## NilRollup

- NilRollup contract is the entrypoint contract for batch commits and proof verifications initiated by syncCommittee

- NilRollup contract contains 2 main functions:
   1. [commitBatch](./contracts/NilRollup.sol#L343)
   2. [updateState](./contracts/NilRollup.sol#L392)

## NilVerifier

1. [NilVerifier-Contract](./contracts/verifier/NilVerifier.sol) contains the [verify](./contracts/verifier/NilVerifier.sol#L9) proof logic
2. NilVerifier contract is non-upgradeable and stateless contract

## Local Development

### Installation

```bash
npm install @nilfoundation/rollup-bridge-contracts
```

### Set environment variables

copy `.env.example` to `.env`

### Compile contracts

```bash
cd rollup-brige-contracts
npx hardhat compile
```

### Deploy contracts to geth instance via nix script:

```bash
npx hardhat deploy --network geth --tags NilContracts
```
