# =nil; cluster

## Description

=nil; is a sharded blockchain whose global state is split between several execution shards. Execution shards are managed by a single master/main shard that references the latest blocks across all execution shards. Each new block produced in an execution shard must also reference the latest block in the main shard.

This project is a prototype implementation of =nil; in Go.

## Building and using the project

### Prerequisites

Install Nix:

```bash
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

### Building and running

Enter the Nix development environment:

```bash
cd nil
nix develop
```

Build the project with:

```bash
make
```

To run the cluster:

```bash
./build/bin/nil
```

To run the load generator:

```bash
./build/bin/nil_load_generator
```

To access the =nil; CLI:

```bash
./build/bin/nil_cli
```

### Running tests

Run tests with:

```bash
make test
```

## Unique features

=nil; boasts several unique features making it distinct from Ethereum and other L2s. 

* [**Structurally distinct external and internal messages**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/core-concepts/shards-parallel-execution#internal-vs-external-messages)
* [**Async execution**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/core-concepts/shards-parallel-execution#async-execution)
* [**Cross-shard communications without fragmentation**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/core-concepts/shards-parallel-execution#message-passing-checks)

## Tools

To interact with the cluster, =nil; supplies several developer tools. 

* The =nil; CLI (provided in this repository)
* [**The `Nil.js` client library**](https://github.com/nilFoundation/nil.js)
* [**The =nil; Hardhat plugin**](https://github.com/NilFoundation/nil-hardhat-plugin)

## =nil; CLI confirugation

The =nil; CLI requires initial setup before being able to interact with the cluster.

To create the config file if it does not already exist:

```bash
./build/bin/nil_cli config config init
```

To configure the CLI:

```bash
./build/bin/nil_cli config set rpc_endpoint NIL_ENDPOINT
./build/bin/nil_cli keygen new
```

This will update the CLI configuration file to include the RPC endpoint and the private key to be used for creatign a wallet and signing messages.

[**This tutorial outlines the steps needed**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/tools/nil-cli/usage) to configure the =nil; CLI.

## Wallets and contracts

In =nil; a wallet is any smart contract that authenticates users and allows for sending signed messages to other contracts. There are no other structural distinctions between a smart contract and a wallet. This means that a wallet can have any logic that a smart contract can have, which makes it easy to create complex wallets (e.g., vesting wallets).

[**Learn more about wallets in =nil;**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/getting-started/essentials/creating-a-wallet).

### Creating a new wallet

The easisest way to create a new wallet is to use the =nil; CLI after configuring it.

To create a new wallet **on the base shard**:

```bash
nil_cli wallet new
```

### Deploying a smart contract

This brief tutorial describes how to deploy the `./examples/counter.sol` contract.

To compile the contract:

```bash
solc -o . --bin --abi example/counter.sol --overwrite
```

The docs contain [**a more detailed tutorial about the different means of contract deployment**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/getting-started/essentials/creating-a-wallet).

To deploy the contract through the wallet:

```bash
./build/bin/nil_cli wallet deploy ./SimpleStorage.bin --abi ./SimpleStorage.abi
```

To deploy the contract through an external message:

```bash
./build/bin/nil_cli contract address ./SimpleStorage.bin
./build/bin/nil_cli wallet send-tokens ADDRESS 50000000
./build/bin/nil_cli contract deploy ./SimpleStorage.bin
```

### Calling a smart contract

The `./example/counter.sol` contains the `increment()` method which can be called in different ways.

**NB**: the `increment()` method modifies the contract state and it cannot be called via an external message.

The docs contain [**a more detailed tutorial on calling smart contract methods**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/getting-started/working-with-smart-contracts/calling-contract-methods).

To call the method via the wallet:

```bash
./build/bin/nil_cli wallet send-message ADDRESS increment --abi ./SimpleStorage.abi
```

### Tokens and multi-currency support

The basic token in =nil; is used for paying for message execution:

* For internal messages, the message itself acts as the 'payer', spending its value
* For internal messages, the 'receiver' contract acts as the 'payer'

In the case of external deployment, funds have to be set for the intended address first before actual deployment occurs. Unless the address already contains some funds, the contract cannot pay for its own external execution.

[**Learn more about the payment structure during external deployment**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/getting-started/working-with-smart-contracts/deploying-a-contract#external-deployment).

=nil; also has a multi-currency mechanism. All accounts (smart contracts) can contain any number of arbitrary currencies as a Merkle trie root. Currency creation is dedicated to a special precompiled contract (the minter), and anyone can request the creation of new currencies. 

**NB**: the currency owner is recorded during creation, and only messages from the owner are processed for the currency. A contract can only be the owner of one currency.

The documentation contains [**an extensive tutorial on working with custom currencies**](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/getting-started/essentials/tokens-multi-currency).  

### Generating the SSZ serialization code

Run the below command to generate the SSZ serialization code:

```bash
make ssz
```

### Generating zero state compiled contracts code

```bash
make compile-contracts
```

## Lifecycle

In the current implementation, a message passes the following stages.

1. The message is submitted by a user (external message) or a smart contract (internal message)

2. If the message is external, it is added to the mempool of its destination shard

3. The message is picked is picked up by the collator, which is the component responsible for passing messages for execution

4. The message is batched with other messages by the collator, and is subsequently executed

5. The message is included in the new block within its destination shard

## The DB

The implementation uses BadgerDB which is a database engine written in pure Go.

BadgerDB boasts an LSM tree design with a value log. The engine is also optimized for SSDs, making it the perfect choice for the cluster.

The DB supports performing CRUD operations on individual sharded tables (e.g., `PutToShard()` or `DeleteFromShard()`).

## The RPC

The current RPC is loosely modeled after the Ethereum RPC. The RPC exposes the following methods.

### Blocks

* `GetBlockByNumber()`
* `GetBlockByHash()`
* `GetBlockTransactionCountByNumber()`
* `GetBlockTransactionCountByHash()`

### Messages

* `GetInMessageByHash()`
* `GetInMessageByBlockHashAndIndex()`
* `GetInMessageByBlockNumberAndIndex()`
* `GetRawInMessageByBlockNumberAndIndex()`
* `GetRawInMessageByBlockHashAndIndex()`
* `GetRawInMessageByHash()`

### Receipts

* `GetInMessageReceipt()`

### Accounts

* `GetBalance()`
* `GetCode()`
* `GetTransactionCount()`
* `GetCurrencies()`

### Transactions

* `SendRawTransaction()`

### Filters

* `NewFilter() `
* `NewPendingTransactionFilter()`
* `NewBlockFilter()`
* `UninstallFilter()`
* `GetFilterChanges()`
* `GetFilterLogs()`
* `GetShardIdList()`

### Shards

* `GetShardIdList()`

### Calls

* `Call()`

### Chains

* `ChainId()`

## OpenRPC spec generator

The project also includes a generator of an OpenRPC spec file from the type definitions the RPC API interface.

The primary benefit of this is allowing for automatic RPC API documentation generation on the side of [**the documentation portal**](https://docs.nil.foundation/).

Another benefit is greater coupling of docs and code. Do not hesitate to adjust the doc strings (be mindful to follow the doc string spec) in `rpc/jsonrpc/eth_api.go`, `rpc/jsonrpc/types.go` and `rpc/jsonrpc/doc.go` to account for latest changes in the RPC API. All changes will make their way to the documentation portal without any overhead.

To run the spec generator:

```bash
cp cmd/spec_generator/spec_generator.go .
go run spec_generator.
rm spec_generator.go
```

This will procude the `openrpc.json` file in the root directory.

The spec generator is part of CI, and the OpenRPC spec will be hosted at the devstand.

## Linting

The project uses `golangci-lint`, a linter runner for Go.

All linters are downloaded and built as part of the `nix develop` command. Run linters with:

```bash
make lint
```

`.golangci.yml` contains the configuration for `golangci-lint`, including the full list of all linters used in the project. [**Visit the official docs for `golangci-lint`**](https://golangci-lint.run/usage/linters).

Additional guides on integrating linters with IDEs:

* **https://github.com/mvdan/gofumpt?tab=readme-ov-file#installation**

* **https://golangci-lint.run/welcome/integrations/**

* **https://github.com/luw2007/gci?tab=readme-ov-file#installation**

## Packaging

Create a platform-agnostic deb package:

```
nix bundle --bundler . .#nil
```
