# =nil; cluster

## Description

=nil; is a sharded blockchain whose global state is split between several execution shards. Execution shards are managed by a single main shard that references the latest blocks across all execution shards. Each new block produced in an execution shard must also reference the latest block in the main shard.

This project is an implementation of =nil; in Go.

<p align="center">
  <a href="https://docs.nil.foundation"><strong>Documentation</strong></a> ·
  <a href="https://explore.nil.foundation/"><strong>Block explorer</strong></a> ·
  <a href="https://explore.nil.foundation/playground"><strong>Playground</strong></a>
</p>

## Table of contents

* [Building and using the project](#building-and-using-the-project)
* [Unique features](#unique-features)
* [Repository structure](#repository-structure)
* [The RPC](#the-rpc)
* [Open RPC spec generator](#openrpc-spec-generator)
* [Linting](#linting)
* [Packaging](#packaging)
* [Debugging](#debugging)

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

Build the project and binaries with:

```bash
make
```

To run the cluster:

```bash
./build/bin/nild run --http-port 8529
```

To run the [faucet service](https://docs.nil.foundation/nil/getting-started/essentials/tokens#token-faucet-service):

```bash
./build/bin/faucet run
```

To run the [Cometa service](https://docs.nil.foundation/nil/guides/cometa-and-debugging):

```bash
./build/bin/cometa run
```

To run the load generator:

```bash
./build/bin/nil_load_generator
```

To access the =nil; CLI:

```bash
./build/bin/nil
```

### Using Nix

The repository uses Nix for dependency and project management. 

To enter the Nix development environment:

```bash
nix develop .#DERIVATION_NAME
```

To build a project according to its derivation:

```bash
nix build .#DERIVATION_NAME
```

### Using NPM

The repository uses NPM Workspaces to manage collective dependencies across the JS/TS projects it is hosting. 

There can only be one top-level `package-lock.json` file that can be regenerated as follows:

```bash
npm run install:clean
```

Individual projects should not have separate `package-lock.json` files. If such files exist, it may lead to unintended behaviors.

In addition, Nix validates the hashum of the `package-lock.json` file when building the project. Perform the following actions after running the `install:clean` script:

1. Open the `./nix/npmdeps.nix` file
2. Remove the current hashsum (located under the list of all `package.json` files)
3. Attempt to enter the Nix environment by running `nix develop .#DERIVATION_NAME`
4. Wait for Nix to provide the correct hash
5. Place the correct hash inside the `./nix/npmdeps.nix` file

### Running tests

Run tests with:

```bash
make test
```

### Generating the SSZ serialization code

Run the below command to generate the SSZ serialization code:

```bash
make ssz
```

### Generating zero state compiled contracts code

```bash
make compile-contracts
```

## Unique features

=nil; boasts several unique features making it distinct from Ethereum and other L2s.

* [Structurally distinct external and internal transactions](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/core-concepts/shards-parallel-execution#internal-vs-external-transactions)
* [Async execution](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/core-concepts/shards-parallel-execution#async-execution)
* [Cross-shard communications without fragmentation](https://docs-nil-foundation-git-nil-ethcc-nilfoundation.vercel.app/nil/core-concepts/shards-parallel-execution#transaction-passing-checks)

## Repository structure

The cluster source code is available at `./nil`.

To interact with the cluster, =nil; supplies several developer tools.

* The =nil; CLI (`./nil/cmd/nil`)
* The `Nil.js` client library (`./niljs`)
* A generator for pre-configured Hardhat projects (`./create-nil-hardhat-project`)
* The block explorer and the Playground (`./explorer_backend` and `./explorer_frontend`)
* The `smart-contracts` NPM package containing Solidity libraries for interacting with =nil;

The repository also houses the following projects:

* `./clijs`, a re-write of the =nil; CLI using JS/TS
* `./docs`, the =nil; documentation available at https://docs.nil.foundation
* `./docs_ai_backend`, a Next.js app handling the RAG chatbot available inside the =nil; documentation
* `./explorer_frontend` and `./explorer_backend`, the two core components of the =nil; block explorer and Playground
* `./l1-contracts`, the =nil; contracts to be deployed on Ethereum
* `./uniswap`, an implementation of the Uniswap V2 protocol on =nil;

The `./nix` folder houses Nix derivations.

## The RPC

The current RPC is loosely modeled after the Ethereum RPC. The RPC exposes the following methods.

### Blocks

* `GetBlockByNumber()`
* `GetBlockByHash()`
* `GetBlockTransactionCountByNumber()`
* `GetBlockTransactionCountByHash()`

### Transactions

* `GetInTransactionByHash()`
* `GetInTransactionByBlockHashAndIndex()`
* `GetInTransactionByBlockNumberAndIndex()`
* `GetRawInTransactionByBlockNumberAndIndex()`
* `GetRawInTransactionByBlockHashAndIndex()`
* `GetRawInTransactionByHash()`

### Receipts

* `GetInTransactionReceipt()`

### Accounts

* `GetBalance()`
* `GetCode()`
* `GetTransactionCount()`
* `GetTokens()`

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

The project also includes a generator of an OpenRPC spec file from the type definitions of the RPC API interface.

The primary benefit of this is allowing for automatic RPC API documentation
generation on the side of [the documentation portal](https://docs.nil.foundation/).

Another benefit is greater coupling of docs and code. Do not hesitate to adjust the doc strings (be mindful to follow the doc string spec) in `rpc/jsonrpc/eth_api.go`, `rpc/jsonrpc/types.go` and `rpc/jsonrpc/doc.go` to account for latest changes in the RPC API. All changes will make their way to the documentation portal without any overhead.

To run the spec generator:

```bash
cp nil/cmd/spec_generator/spec_generator.go .
go run spec_generator.
rm spec_generator.go
```

This will produce the `openrpc.json` file in the root directory.

## Linting

The project uses `golangci-lint`, a linter runner for Go.

All linters are downloaded and built as part of the `nix develop` command. Run linters with:

```bash
make lint
```

`.golangci.yml` contains the configuration for `golangci-lint`, including the
full list of all linters used in the project.


## Packaging

Create a platform-agnostic deb package:

```
nix bundle --bundler . .#nil
```

## Debugging

### Block replay

=nil; allows for reproducing execution of a particular block. To do so, run the cluster in the block-replay mode:

```bash
nild --db-path ./database replay-block --first-block STARTING_BLOCK --last-block FINAL_BLOCK --shard-id SHARD_ID --log-level trace
```

NB: by default, the replay mode fully copies the existing production DB. It is possible to avoid this by only fetching the required records. Use the read-through mode to do so:

```bash
nild --read-through-db-addr $RPC_ENDPOINT --read-through-fork-main-at-block FORK_NUM replay-block --first-block STARTING_BLOCK --last-block FINAL_BLOCK --shard-id SHARD_ID --log-level trace
```

The `FORK_NUM` placeholder represents the number of blocks beyond which records will not be retrieved from the production DB.
