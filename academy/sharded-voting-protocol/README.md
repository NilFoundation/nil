# 🗳️ Sharded Voting Protocol on =nil

## 🔍 Overview

This repository contains an **educational example** of a decentralized voting application built on the **=nil;** blockchain. The protocol leverages **sharded smart contracts**, **asynchronous cross-shard communication**, and the powerful `nil.sol` infrastructure to enable scalable and efficient voting at large scale.

This project demonstrates how voting data can be split and handled across multiple shards, while the results are asynchronously collected and tallied by a central manager.

### ✨ Features

- 🧩 **Sharded Voting Architecture**: Deploy voting logic across multiple shards
- 📤 **Vote Casting via Manager or Directly**: Vote directly or through the VoteManager
- 🔄 **Asynchronous Vote Tallying**: Collect votes shard-by-shard using async requests
- 📊 **Final Aggregated Results**: Consolidate votes into a final result set

### 🚀 Key Highlights

- ⚙️ **CREATE Deployment**: Deterministic deployment of VoteShard contracts
- 🔁 **Nil.Async Infrastructure**: Use of `Nil.asyncCall` and `Nil.sendRequest` for async ops
- 🔗 **Cross-Shard Coordination**: VoteManager interacts with multiple shards for casting and tallying
- ✅ **Secure Voting Flow**: Ensures contract-only forwarding via `voteManager`

## ⚙️ Prerequisites

To work with this repository, make sure you have the following:

- 📌 [Node.js](https://nodejs.org/) (v16 or later recommended)
- 📦 [npm](https://www.npmjs.com/)
- 🧪 [Hardhat](https://hardhat.org/)
- 🌍 Access to a **=nil; testnet RPC endpoint**  
  👉 Get one via the [=nil; Devnet Bot](https://t.me/NilDevnetTokenBot)
- 🔐 A `.env` file with your RPC and private key config (based on `.env.example`)

Check your setup:

```bash
node -v
npm -v
```

## 📦 Installation

1. 📥 Clone the repository:

   ```sh
   git clone git clone https://github.com/NilFoundation/nil.git
   ```

2. 📂 Navigate to the directory and install dependencies:

   ```sh
   cd nil/academy/sharded-voting-protocol
   npm install
   ```

3. 🗂️ Configure your `.env` file:

   ```sh
   cp .env.example .env
   # Edit `.env` to add your RPC and private key
   ```

4. 🛠 Compile the smart contracts:

   ```sh
   npx hardhat compile
   ```

5. 🚀 Run the full voting workflow:

   ```sh
   npx hardhat run-voting-protocol
   ```

## 🔄 Process Flow

1. **Deploy Voting Shards** via `VoteManager`.
2. Users cast votes directly or through `VoteManager`.
3. Each `VoteShard` stores and tracks its own voters and votes.
4. After the voting period ends, `VoteManager` initiates tallying.
5. Tally results are collected from all shards.
6. Final vote totals are stored and accessible from the `results` mapping.

## 🤝 Contribution

This protocol is a learning tool, but it’s open to improvements from the community!

### 💡 Ideas to Contribute

- 🧠 Add **voting weights** based on token holdings
- 🎯 Implement **per-shard filters** or categories
- 🗂 Improve **tallying performance** with parallel processing

Check our open issues [here](https://github.com/NilFoundation/nil/issues)  
Read our [Contribution Guide](https://github.com/NilFoundation/nil/blob/main/CONTRIBUTION-GUIDE.md)

---

🚀 **Thanks for supporting decentralized development — happy building!** 🎉
