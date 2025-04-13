# Nil Contracts

Smart contracts for communication between L1 and NilChain, enabling proof verification, state root updates, and token bridging.

---

## Contracts Overview

### L1 Contracts

Located in `/contracts`, these Solidity contracts facilitate L2 proof verification and token bridging **from L1 to NilChain**.

- **NilRollup**  
  - Entry point for batch commits and proof verification via `syncCommittee`.
  - Key functions:
    - [`commitBatch`](./contracts/NilRollup.sol#L248)
    - [`updateState`](./contracts/NilRollup.sol#L294)

- **NilVerifier**  
  - Stateless and non-upgradeable proof verifier contract.  
  - Core function: [`verify`](./contracts/verifier/NilVerifier.sol#L9)

- **BridgeContracts (L1)**  
  - Located in [`/contracts/bridge/l1`](./contracts/bridge/l1/)
  - Enable token bridging **between L1 and NilChain**.

---

### L2 Contracts

- Located in [`/contracts/bridge/l2`](./contracts/bridge/l2/)
- Facilitate token bridging **between NilChain and L1**.


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
pnpm clean && npx pnpm compile
```

### Commands to Deploy Contracts

1. clear older deployment config
```sh
npx hardhat run scripts/wiring/clear-deployments.ts --network geth
```

2. Set the properties in [l1-deployment-config.json](./deploy/config/l1-deployment-config.json)
  - chose the nested json object under `geth` and set the properties which start with 
    1. l1DeployerConfig
    2. nilRollupDeployerConfig
    3. nilGasPriceOracleDeployerConfig

```sh
npx hardhat run scripts/wiring/set-deployer-config.ts --network geth
```

3. Deploy Mock and Token contracts

```sh
npx hardhat deploy --network geth --tags DeployL1Mock
```

4. Deploy Rollup and BridgeContracts

```sh
npx hardhat deploy --network geth --tags DeployL1Master
```

### Wire Dependencies

```sh
npx hardhat run scripts/wiring/wiring-master.ts --network geth
```

### Test Deposit ERC20 & ETH

```sh
npx hardhat run scripts/bridge-test/bridge-erc20.ts --network geth
```

```sh
npx hardhat run scripts/bridge-test/bridge-eth.ts --network geth
```