<h1 align="center">create-nil-hardhat-project</h1>

<br />

<p align="center">
  The TypeScript generator of Hardhat projects working on top of =nil;
</p>

## Table of contents

* [Overview](#-overview)
* [Installation](#-installation)
* [Configuration](#️-configuration)
* [Usage](#-usage)
* [Contributing](#-contributing)
* [Work in progress](#-work-in-progress)
* [License](#license)

## 🚀 Overview

This project houses `create-nil-hardhat-project`, a generator of Hardhat projects that work on top of =nil;. 


## 🔧 Installation

Clone the repository:

```bash
git clone https://github.com/NilFoundation/nil.git
cd ./nil/create-nil-hardhat-project
```
Install dependencies:

```bash
pnpm install
```

## ⚙️ Configuration

1. Create a `.env` file in the root directory based on the given `.env.example` file.
2. Update the `.env` file with the RPC URL. The default value corresponds to a locally running =nil; node.
3. Generate a private key and a new smart account using the hardhat default task

```bash
  npx hardhat wallet create
```

4. Update the `.env` file with the private key and the smart account address.


## 🎯 Usage

To compile all smart contracts:

```bash
  npx hardhat compile
```

Create a task for deploying the `Incrementer.sol` contract in `./tasks/deploy-incrementer.ts`:

```ts
import { task } from "hardhat/config";

task("deploy-incrementer").setAction(async (taskArgs, hre) => {
   const contract = await hre.nil.deployContract("Incrementer", []);

   console.log("Incrementer contract deployed at address: " + contract.address);

   await contract.write.increment([]);

   const value = await contract.read.getValue([]);

   console.log("Incrementer contract value: " + value);
});
```

Import the task inside `hardhat.config.ts`:

```ts
import "./tasks/deploy-incrementer";
```

To deploy the contract:

```bash
  npx hardhat deploy-incrementer
```

To run all tests:

```bash
  npm run tests
```

## Methods

Get default client:

```typescript
const client = await hre.nil.getPublicClient();
```

Get default smart account:

```typescript
const smartAccount = await hre.nil.getSmartAccount();
```

Create new smart account:

```typescript
const smartAccount = await hre.nil.createSmartAccount({
  topUp: true, // top up the account with tokens
});
```

Contact deployment with params:

```typescript
const contract = await hre.nil.deployContract("Incrementer", [0n], {
  shardId: 1, // optionally set shard manually
  feeCredits: 100_000_000n, // optionally set max fee credits
});
```

Get and call deployed contract:

```typescript
import { waitTillCompleted } from "./receipt";

const tokenContract = await hre.nil.getContractAt("Token", token0Address, {
  smartAccount: smartAccount, // optionally set smart account for method calls
});

const txHash = await tokenContract.write.sendToken(["<to_addr", 1000n]);

// wait for the transaction to be completed
await waitTillCompleted(client, tx, {
  waitTillMainShard: true, // wait for the main shard (useful for cross-shard transactions)
});

const balanace: bigint = await tokenContract.read.getTokenBalanceOf(["<address>"]);
```

## 💪 Contributing

Contributions are always welcome! Feel free to submit pull requests or open issues to discuss potential changes or improvements.

## 🚧 Work in Progress

This project is currently under active development. Not all features are fully implemented and the project may behave unexpectedly in certain cases. If an issue arises when interacting with the project, open a new GitHub issue and detail the problem.

## License

[MIT](./LICENCE)
