# Lending and Borrowing Protocol on =nil;

This repository contains a demo project showcasing a lending and borrowing protocol built on the =nil; blockchain. The project includes smart contracts and integration scripts to help developers understand how to work with the =nil; ecosystem, sharded smart contracts, and cross-shard interactions.

## Overview

The protocol allows users to:

- Deposit USDT and ETH into a lending pool
- Borrow assets based on provided collateral
- Repay borrowed assets
- Utilize an oracle for price updates

### Contracts Deployed

1. **InterestManager** - Manages interest calculations.
2. **GlobalLedger**  - Stores deposit balances and loan tracking.
3. **Oracle** - Provides asset prices.
4. **LendingPool**  - Facilitates deposits, borrowing, and repayments.

## End-to-End Flow

The provided integration script executes the following steps:

1. Deploys all contracts across different shards.
2. Creates two smart contract accounts.
3. Sets and verifies oracle prices for assets.
4. Tops up the accounts with USDT and ETH.
5. Deposits funds into the lending pool.
6. Borrows ETH against deposited USDT.
7. Repays borrowed ETH.

## Getting Started

### Prerequisites

- Node.js (>=16.x)
- Hardhat
- A =nil; testnet RPC endpoint
- `.env` file with reference to `.env.example`

### Installation

```sh
npm install
```

### Running the End-to-End Test

Compile the contracts:

```
npx hardhat compile
```

Run the below command and see the logs for the interactions:

```sh
npx hardhat e2eInteraction
```

This script will execute the full lending and borrowing workflow on the =nil; blockchain.

## Smart Contracts

### LendingPool.sol

Handles deposits, withdrawals, borrowing, and repayments. It interacts with the GlobalLedger for storage and the Oracle for asset prices.

### InterestManager.sol

Calculates interest rates.

### GlobalLedger.sol

Stores balances and loan records for users.

### Oracle.sol

Manages asset price updates and queries.

## Benefits of This Protocol

- **Sharding Efficiency:** The use of sharded contracts improves scalability by distributing workloads across different shards.
- **Asynchronous Execution:** Enables better parallel processing, reducing bottlenecks in transactions.
- **Cross-Shard Communication:** The protocol demonstrates how smart contracts can interact across different shards for complex financial operations.

## Things you can work on:

- Implement interest rate models based on utilization.
- Add liquidation mechanisms for undercollateralized loans.
- Enable additional assets and multi-collateral support.
- Add  a feature for permissionless addition of pools by any users.
- Add secured mechanism for state changes related to collateral manager
- Ability to deposit on lending pool on a given shard and borrowing from a lending pool on other shards.

## Learning & Contribution

By going through the code and understanding how it works, developers can learn how to build on the =nil; blockchain, taking advantage of sharding, asynchronous execution, and efficient contract design. We welcome contributions! Please refer to the [Contribution Guide](CONTRIBUTING.md) for details on submitting PRs and reporting issues.


