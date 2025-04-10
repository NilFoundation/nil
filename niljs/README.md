<h1 align="center">Nil.js</h1>

<br />

<p align="center">
  The TypeScript client library for interacting with the =nil; cluster.
</p>

<row style="display: flex; gap: 10px;"><p align="center">
<a href="https://github.com/NilFoundation/nil.js/actions/workflows/build.yaml">
<picture>
<img src="https://img.shields.io/github/actions/workflow/status/NilFoundation/nil.js/.github%2Fworkflows%2Fbuild.yaml"/>
</picture>
</a>
<a href="https://www.npmjs.com/package/@nilfoundation/niljs">
<picture>
<img src="https://img.shields.io/npm/dy/%40nilfoundation%2Fniljs"/>
</picture>
</a>
<a href="https://github.com/NilFoundation/nil.js">
<picture>
<img src="https://img.shields.io/github/stars/NilFoundation/nil.js"/>
</picture>
</a>
<a href="https://github.com/NilFoundation/nil.js/actions/workflows/build.yaml">
<picture>
<img src="https://img.shields.io/npm/v/%40nilfoundation%2Fniljs"/>
</picture>
</a>
<a href="https://github.com/NilFoundation/nil.js">
<picture>
<img src="https://img.shields.io/github/forks/NilFoundation/nil.js"/>
</picture>
</a>

</p>
</row>

## Table of contents

- [Installation](#installation)
- [Getting started](#getting-started)
- [Usage](#usage)
- [Tokens and bouncing](#tokens-and-bouncing)
- [Accessing the dev environment](#accessing-the-dev-environment)
- [Licence](#licence)

## Installation

```bash
npm install @nilfoundation/niljs
```

## Getting started

`PublicClient` is used for performing read-only requests to =nil; that do not
require authentication (e.g., attaining information about a block).

To initialize a `PublicClient`:

```typescript
const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: RPC_ENDPOINT,
  }),
  shardId: 1,
});
```

`shardId` is a concept unique to =nil; in that it designates the execution shard
where the smart account should be deployed. Execution shards manage portions of
the global state and are coordinated by the main shard.

`SmartAccountV1` is a class representing a smart account that allows for signing
transactions and performing requests that require authentication.

To deploy a new smart account:

```typescript
const smartAccount = await generateSmartAccount({
  shardId: 1,
  rpcEndpoint: RPC_ENDPOINT,
  faucetEndpoint: FAUCET_ENDPOINT,
});
```

## Usage

In =nil;, it is possible to call functions asynchronously. When a contract makes
an async call, a new transaction is spawned. When this transaction is processed,
the function call itself is executed.

It is possible to make async calls within the confines of the same shard or
between contracts deployed on different shards.

To perform an async call:

```typescript
const anotherAddress = SmartAccountV1.calculateSmartAccountAddress({
  pubKey: pubkey,
  shardId: 1,
  salt: 200n,
});

await smartAccount.sendTransaction({
  to: anotherAddress,
  value: 10n,
  gas: 100000n,
});
```

To perform a sync call:

```typescript
const anotherAddress = SmartAccountV1.calculateSmartAccountAddress({
  pubKey: pubkey,
  shardId: 1,
  salt: 200n,
});

await smartAccount.syncSendTransaction({
  to: anotherAddress,
  value: 10n,
  gas: 100000n,
});
```

It is only possible to perform sync calls within the confines of one shard.

## Tokens and bouncing

=nil; provides a multi-token mechanism. A contract can be the owner of one
custom token, and owners can freely send custom tokens to other contracts. As a
result, the balance of a given contract may contain standard tokens, and several
custom tokens created by other contracts.

Custom tokens do not have to be created, and each contract is assigned one by
default. However, at contract deployment, a token has no name and its total
supply equals zero.

To set the name of the token for an existing smart account:

```ts
const transaction = await smartAccount.sendTransaction({
  to: smartAccountAddress,
  feeCredit: 1_000_000n * gasPrice,
  value: 0n,
  data: encodeFunctionData({
    abi: SmartAccountV1.abi,
    functionName: "setTokenName",
    args: ["MY_TOKEN"],
  }),
});

await transaction.wait();
```

To mint 1000 tokens:

```ts
const transaction2 = await smartAccount.sendTransaction({
  to: smartAccountAddress,
  feeCredit: 1_000_000n * gasPrice,
  value: 0n,
  data: encodeFunctionData({
    abi: SmartAccountV1.abi,
    functionName: "mintToken",
    args: [100_000_000n],
  }),
});

await transaction2.wait();
```

To send a token to another contract:

```ts
const anotherAddress = generateRandomAddress();

const tx = await smartAccount.sendTransaction({
  to: anotherAddress,
  value: 10_000_000n,
  feeCredit: 100_000n * gasPrice,
  tokens: [
    {
      id: smartAccountAddress,
      amount: 100_00n,
    },
  ],
});

await tx.wait();
```

=nil; also supports token bouncing. If a transaction carries custom tokens, and
it is unsuccesful, the funds will be returned to the address specified in the
`bounceTo` parameter when sending the transaction.

## Accessing the dev environment

To enter the Nix dev environment for `Nil.js`:

```bash
nix develop .#niljs
```

After that, it should be possible to run `npm run test` and other scripts
specified in `./niljs/package.json`.

## Licence

[MIT](./LICENCE)
