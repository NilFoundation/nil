# =nil; cluster

## Description

=nil; is a sharded blockchain whose global state is split between several execution shards. Execution shards are managed by a single master/main shard that references the latest blocks across all execution shards. Each new block produced in an execution shard must also reference the latest block in the main shard.

This project is a prototype implementation of =nil; in Go.

## Building and using the project

### Prerequisites

Install Nix:

#### On Linux

```bash
sh <(curl -L https://nixos.org/nix/install) --daemon
```

#### On Mac

```bash
sh <(curl -L https://nixos.org/nix/install)
```

#### On Mac silicon

```bash
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

### Building

Build the project with:

```bash
cd nil
make
```

### Running tests

Run tests with:

```bash
make test
```

### Deploying a smart contract

Create a deployment message:

```golang
var m types.Message
m.From = common.GenerateRandomAddress(uint32(types.MasterShardId))
dm := types.DeployMessage{
    ShardId: uint32(types.MasterShardId), 
    Code: hexutil.FromHex("{contractBytecode}")
    }
data, err := dm.MarshalSSZ()
suite.Require().NoError(err)
m.Data = data
mData, err := m.MarshallSSZ()
```

Send the deployment message:

```golang
request := Request{
    Jsonrpc: "2.0",
    Method:  sendRawTransaction,
    Params:  []any{"0x" + hex.EncodeToString(mData)},
    Id:      1,
}

resp, err := makeRequest[common.Hash](&request)
```

Create the address:

```golang
addr := common.CreateAddress(uint32(types.MasterShardId), m.From, m.Seqno)
```

Send repeated requests for receipts:

```golang
request.Method = getMessageReceipt
request.Params = []any{types.MasterShardId, msgHash}

var respReceipt *Response[*types.Receipt]
suite.Eventually(func() bool {
		respReceipt, err = makeRequest[*types.Receipt](&request)
		suite.Require().NoError(err)
		suite.Require().Nil(resp.Error["code"])
		return respReceipt.Result != nil
	}, 
    5*time.Second, 
    200*time.Millisecond)
```

### Calling a deployed smart contract

Extract the contract bytecode:

```golang
codeHex = hexutil.Encode(contractBytecode)
m.From = common.HexToAddress(codeHex)
```

Create a contract call message:

```golang
methodData, _ := parsedABI.Pack("{methodName}")
m.Data = methodData
m.Seqno = 1
m.To = addr
mData, err = m.MarshalSSZ()
```

Send the message:

```golang
newRequest := Request {
    Jsonrpc: "2.0",
    Method: sendRawTransaction,
    Params: []any{"0x" + hex.EncodeToString(mData)},
    Id: 1,
newResp, err := makeRequest(&newRequest)
}

txHash = result["hash"].(string)

time.Sleep(2 * time.Second)
```

Get the receipt:

```golang
newRequest.Method = getMessageReceipt
newRequest.Params = []any{types.MasterShardId, txHash}
respTwo, err = makeRequest(&request)

receipt, err = types.FromMap[types.Receipt](result)
```

### Generating the SSZ serialization code

Run the below command to generate the SSZ serialization code:

```bash
make ssz
```

## Lifecycle

In the current implementation, a transaction passes the following stages.

1. The transaction is submitted by an account

The RPC exposes the `SendRawTransaction()` method that calls the `CreateRwTx()` function of the DB and adds the transaction to the mempool.

2. The transaction is sent to the mempool of the shard where it originated

The `msgpool` object calls the `Add()` function to iterate over a list of transactions and add them to the mempool. The function also returns a list of reasons for discarding a transaction (if any were discarded).

3. The transaction is picked up by the collator, which is the component responsible for passing transactions for execution

The collator periodically calls the `GenerateBlock()` method of the shardchain object and the `OnNewBlock()` handler of the mempool.

4. The shard executes the transaction

The shardchain records the block with the transaction in the DB and updates the execution state of the shard.

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

### Receipts

* `GetInMessageReceipt()`

### Accounts

* `GetBalance()`
* `GetCode()`
* `GetTransactionCount()`

### Transactions

* `SendRawTransaction()`

### Logs

* `NewFilter() `
* `NewPendingTransactionFilter()`
* `NewBlockFilter()`
* `UninstallFilter()`
* `GetFilterChanges()`
* `GetFilterLogs()`
* `GetShardIdList()`

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
