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
3. Generate a private key and a new smart account using the =nil; CLI:

   ```bash
   nil keygen new
   nil smart-account new
   ```

4. Update the `.env` file with the private key and the smart account address.


## 🎯 Usage

To compile all smart contracts:

```bash
npx run compile
```

Create a task for deploying the `Incrementer.sol` contract in `./tasks/deploy-incrementer.ts`:

```ts
import type { Abi } from "abitype";
import { task } from "hardhat/config";
import { createSmartAccount } from "../src/smart-account";
import { deployNilContract } from "../src/deploy";

task("deploy-incrementer").setAction(async (taskArgs, _) => {
  const smartAccount = await createSmartAccount();

  const IncrementerJson = require("../artifacts/contracts/Incrementer.sol/Incrementer.json");

  const { contract, address } = await deployNilContract(
    smartAccount,
    IncrementerJson.abi as Abi,
    IncrementerJson.bytecode,
    [],
    smartAccount.shardId,
    [],
  );
  console.log("Incrementer factory contract deployed at address: " + address);
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

## 💪 Contributing

Contributions are always welcome! Feel free to submit pull requests or open issues to discuss potential changes or improvements.

## 🚧 Work in Progress

This project is currently under active development. Not all features are fully implemented and the project may behave unexpectedly in certain cases. If an issue arises when interacting with the project, open a new GitHub issue and detail the problem.

## License

[MIT](./LICENCE)
