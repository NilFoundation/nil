
<div align="center">
  <h1>nil-hardhat-plugin</h1>
</div>

## 📋 Overview
The `nil-hardhat-plugin` is a custom Hardhat plugin designed to seamlessly integrate with the =nil; blockchain, an Ethereum Layer 2 solution utilizing zkSharding. Traditional development tools like Hardhat are not immediately compatible with =nil; due to its unique features and enhancements.

### 🚧 Challenges with =nil;
DApp developers are accustomed to using Hardhat for deploying and interacting with smart contracts. However, =nil;'s architecture introduces several new concepts that are not native to traditional EVM-compatible networks:
1. Modified RPC methods to suit specific needs of =nil;.
2. Implementation of internal and external messages.
3. Asynchronous communication between shards.
4. Introduction of shards within the blockchain.
5. Transition from externally owned accounts (EOAs) to wallet-based accounts.

### 🛠️ Plugin Functionality
This plugin acts as an adapter, intercepting requests from the Hardhat library to:
1. **Prepare Inputs for =nil; Cluster:** Adjusts the inputs of Hardhat commands to be compatible with =nil;'s requirements.
2. **Adapt Responses for Hardhat:** Ensures the responses from =nil; are reformatted to meet the expectations of the Hardhat framework.

Additionally, the plugin leverages [`nil.js`](https://github.com/NilFoundation/nil.js) to handle specific workflows altered in =nil;  such as the concept of wallets and the deployment and interaction flow of contracts.

## 🎯 Purpose
The aim is to enhance user experience and developer onboarding by allowing developers to integrate this plugin into their projects seamlessly. This approach ensures that developers can continue to use their familiar tools while taking full advantage of =nil;'s advanced features.

## 🤝 Contributing
We welcome contributions from the community to make this plugin even better. If you have suggestions or improvements, please submit a pull request or open an issue. Your input is valuable to us!

