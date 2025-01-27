<h1 align="center">Uniswap v2 =nil;</h1>

<br />

<p align="center">
  An implementation of the Uniswap v2 protocol running on top of =nil;
</p>

## Table of contents

* [Overview]

## 📋 Overview

This project showcases the migration of Uniswap V2 to the =nil; cluster. It adapts Uniswap’s core contracts (Factory, Pair and Router) to work seamlessly with the unique architecture of =nil;. The project can be used as an example of migrating a dApp from Ethereum compatible networks to =nil; while benefitting from improved scalability offered by zkSharding:

1. **Multi-Token Support:** =nil; natively supports multiple custom tokens without having to rely on the ERC20 standard
2. **Async/Sync Calls:** =nil; supports async execution of transactions within one shard or across multiple shards
3. **Load Distribution:** =nil; spreads execution of transactions across shards without fragmenting its state


## ⚙️ Installation

Clone the repository:

```bash
git clone https://github.com/NilFoundation/nil.git
cd ./nil/uniswap
```
Install dependencies:

```bash
npm install
```

Create a new `.env` file structured similarly to [`./.env.example`](./.env.example).

## 🪓 Usage with `Nil.js`

`Nil.js` is the recommended way for interacting with =nil;. To install `Nil.js`:

```bash
npm install @nilfoundation/niljs
```

Create a new smart account: 

```typescript
const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: RPC_ENDPOINT,
  }),
  shardId: 1,
});

const smartAccount = await generateSmartAccount({
  shardId: 1,
  rpcEndpoint: RPC_ENDPOINT,
  faucetEndpoint: FAUCET_ENDPOINT,
});

```

The `SmartAccount` instance can deploy contracts or create contract instances attached to already deployed smart contracts:
```typescript
// deployment
const {contract, address} = await deployNilContract(
  smartAccount,
  FactoryJson.abi,
  FactoryJsn.bytecode,
  [], // constructor arguments
  smartAccount.shardId,
  ["callExternalMethod"], // external methods
);

// attach to an existing contract
const contract2 = getContract({
   abi: TokenJson.abi,
   address: tokenAddress,
   client: smartAccount.client,
   smartAccount: smartAccount, // optional for write methods
   externalInterface: { // optional for external methods
      signer: smartAccount.signer,
      methods: ["callExternalMethod"],
   }
});
```

Call Uniswap contracts after deploying all contracts:

```typescript
// read method
const balance = await contract.read.getTokenBalanceOf([smartAccount.address]);

// write method with tokens
const hash = await pair.write.swap([0, expectedOutputAmount, smartAccount.address], {
   tokens: [{
      id: token0Address,
      amount: swapAmount,
   }]
});

// external methods
const hash1 = await contract.external.mintToken([mintAmount]);
```

## 🎯 Hardhat tasks

The project also provides two Hardhat tasks that showcase the full full lifecycle from deployment to execution. These tasks cover deploying and initializing all necessary contracts as well as minting, swapping, and burning

### Demo Tasks

1. **Using Factory, Pair, and Router Contracts**
   This demo includes an additional layer by utilizing the Router contract along with Factory and Pair
   [View the demo-router task](./tasks/uniswap/demo-router.ts)
   ![alt text](/public/demo-router.png)

Note that:

- The `UniswapV2Factory` is used for creating new pairs. `UniswapV2Router01` calls already deployed pair contracts.
- `UniswapV2Router01` can be deployed on a different shard.
- Vulnerability: no checks are performed during adding/removing liquidity and swaps.
- Rates and output amounts are entirely calculated on the user side.

2. **Using Router with Sync Calls (1 shard)**
   This demo task shows how to deploy the `UniswapV2Router01` contract and use it as a proxy for adding/removing liquidity and swaps via sync calls. It allows for checking amounts before pair calls and maintains token rates.
   [View the demo-router task](./tasks/uniswap/demo-router-sync.ts)

Note that:

- `UniswapV2Router01` should be deployed on the same shard as the pair contract.
- It maintains the token exchange rate when adding/removing liquidity.
- It supports limit checks for token amounts.


### Running the Demo Tasks

1. Compile the project

```bash
npx hardhat compile
```

2. Run the demo tasks:

- For the demo with Router (Factory, Pair, and Router):

```bash
npx hardhat demo-router
```

- For the demo with Router (Sync calls):

```bash
npx hardhat demo-router-sync
```

## 🤝 Contributing

Contributions are always welcome! Feel free to submit pull requests or open issues to discuss potential changes or improvements.

## License

This project is licensed under the GPL-3.0 License. See the [LICENSE](./LICENSE) file for more details. Portions of this project are derived from [Uniswap V2](https://github.com/Uniswap/v2-core) and are also subject to the GPL-3.0 License.
