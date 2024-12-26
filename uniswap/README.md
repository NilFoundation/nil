
<div align="center">
  <h1>Uniswap V2 =nil;</h1>
</div>

## 📋 Overview

This repository showcases the migration of Uniswap V2 to the =nil; We have adapted Uniswap’s core contracts — Factory, Pair, and Router — to work seamlessly with the unique capabilities of =nil;
By using this example, developers can learn how to migrate dApps from Ethereum compatible networks to the =nil; and take advantage of its features, such as:
1. **Multi-Currency Support:** Learn how =nil; supports multiple currencies natively and how they replace the ERC20 standard
2. **Async/Sync Calls:** Discover how to effectively utilize async and sync messaging between shards, making the protocol both scalable and performant
3. **Load Distribution:** Understand how to distribute operations across multiple shards, enhancing parallel processing and scalability

This repository also aims to spark discussion on the potential of running DeFi protocols on sharded architectures, demonstrating their feasibility and advantages

## ⚙️ Prerequirements

Before working with this repository, ensure that you have properly set up your environment:

1. **.env Configuration**:
   Make sure to create a `.env` file in the root directory. All required environment variables are listed in the `.env.example` file, which you can use as a reference. [Link to `.env.example`](./.env.example)

2. **Getting an RPC Endpoint**:
   To request an RPC endpoint:
   - Join our Telegram chat: [https://t.me/nilfoundation](https://t.me/nilfoundation)
   - Request access, and our DevRel team will assist you.

3. **Obtaining a Private Key and Wallet**:
   - Download the `nil` CLI: [https://github.com/NilFoundation/nil_cli](https://github.com/NilFoundation/nil_cli)
   - Follow the setup instructions in our documentation: [nil CLI Docs](https://docs.nil.foundation/nil/getting-started/nil-101)

## 🪓 Usage with nil.js

To interact with the =nil; network, we recommend using the `nil.js` library.
It provides a simple and intuitive way to interact with the network, including sending transactions, querying data, and more.

To get started with `nil.js`, install the package via npm:
```bash
  npm install @nilfoundation/niljs
```

You can initialize `Wallet` and `PublicClient` instances to interact with the network:
```typescript
const wallet = await createWallet({
   faucetDeposit: true, // default: false
});
```

With the `Wallet` instance, you can deploy contracts or attach to existing ones:
```typescript
// deployment
const {contract, address} = await deployNilContract(
        wallet,
        FactoryJson.abi,
        FactoryJsn.bytecode,
        [], // constructor arguments
        wallet.shardId,
        ["callExternalMethod"], // external methods
);

// attach to an existing contract
const contract2 = getContract({
   abi: CurrencyJson.abi,
   address: currencyAddress,
   client: wallet.client,
   wallet: wallet, // optional for write methods
   externalInterface: { // optional for external methods
      signer: wallet.signer,
      methods: ["callExternalMethod"],
   }
});
```

After contract object initialization, you can call its methods:
```typescript
// read method
const balance = await contract.read.getCurrencyBalanceOf([wallet.address]);

// write method with tokens
const hash = await pair.write.swap([0, expectedOutputAmount, wallet.address], {
   tokens: [{
      id: currency0Address,
      amount: swapAmount,
   }]
});

// external methods
const hash1 = await contract.external.mintCurrency([mintAmount]);
```

## 🎯 Usage

To help you get started quickly, we provide two demo tasks that showcase the full lifecycle from deployment to execution. These tasks cover deploying and initializing all necessary contracts,
as well as performing operations like minting, swapping, and burning

### Demo Tasks
1. **Using Factory and Pair Contracts Only**
   This demo handles deploying the Factory and Pair contracts and executing a complete flow of operations
   [View the demo task](https://github.com/NilFoundation/uniswap-v2-nil/blob/main/tasks/core/demo.ts)
   ![alt text](/public/demo.png)


**Important:**
- Calculations are processed on the user's side.
- Both the currency address and its ID are stored in the pair contract.


2. **Using Factory, Pair, and Router Contracts**
   This demo includes an additional layer by utilizing the Router contract along with Factory and Pair
   [View the demo-router task](https://github.com/NilFoundation/uniswap-v2-nil/blob/main/tasks/core/demo-router.ts)
   ![alt text](/public/demo-router.png)

   **Important:**
   - The `UniswapV2Factory` is used for creating new pairs. `UniswapV2Router01` calls already deployed pair contracts.
   - `UniswapV2Router01` can be deployed on a different shard.
   - Vulnerability: no checks are performed during adding/removing liquidity and swaps.
     Rates and output amounts are entirely calculated on the user side.


3. **Using Router with Sync Calls (1 shard)**
   This demo task shows how to deploy the `UniswapV2Router01` contract
   and use it as a proxy for adding/removing liquidity and swaps via sync calls.
   It allows checks on amounts before pair calls and maintains currency rates.
   [View the demo-router task](https://github.com/NilFoundation/uniswap-v2-nil/blob/main/tasks/core/demo-router-sync.ts)

   **Important:**
   - `UniswapV2Router01` should be deployed on the same shard as the pair contract.
   - It maintains the currency exchange rate when adding/removing liquidity.
   - It supports limit checks for currency amounts.


### Running the Demo Tasks
1. **Compile the Project**:
   ```bash
   npx hardhat compile
   ```
2. **Run the Demo Tasks**:
   - For the core demo (Factory and Pair):
     ```bash
     npx hardhat demo 
     ```
   - For the demo with Router (Factory, Pair, and Router):
     ```bash
     npx hardhat demo-router
     ```
   - For the demo with Router (Sync calls):
     ```bash
     npx hardhat demo-router-sync
     ```

### Manual Setup
If you prefer to run everything manually, we provide Ignition modules for each contract:
[Ignition Modules](https://github.com/NilFoundation/uniswap-v2-nil/tree/main/ignition)

Additionally, all crucial contract methods have corresponding tasks:
[Tasks](https://github.com/NilFoundation/uniswap-v2-nil/tree/main/tasks)

Each subdirectory contains a README explaining the details of the tasks

## 🤝 Contributing

We welcome contributions from the community to make this project even better! If you have suggestions, improvements, or find any bugs, please feel free to submit a pull request or open an issue.

Check out our open issues and improvements in the [GitHub Issues](https://github.com/NilFoundation/uniswap-v2-nil/issues) section. If you're new to the project, look for issues labeled as `good first issue` — they're a great place to start.

Your input and contributions are greatly appreciated!

## License
This project is licensed under the GPL-3.0 License. See the [LICENSE](./LICENSE) file for more details. Portions of this project are derived from [Uniswap V2](https://github.com/Uniswap/v2-core) and are also subject to the GPL-3.0 License.
