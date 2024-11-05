
# 🚀 Overview
This repository demonstrates how to deploy and interact with smart contracts on the =nil; blockchain using Hardhat and our custom plugin. The =nil; blockchain is an Ethereum Layer 2 solution based on zk-sharding, enhancing transaction efficiency and scalability.

## 🔧 Installation
Clone the Repository:
```bash
git clone https://github.com/NilFoundation/nil-hardhat-example.git
cd nil-hardhat-example
```
Install Dependencies:
```bash
npm install
```

## ⚙️ Configuration
1. Create a `.env` file in the root directory based on the given `.env.example` file.
2. Update the `.env` file with the RPC URL. The default value corresponds to a locally running =nil; node.
3. Generate a private key and wallet using `nil_cli`:
   ```bash
   nil_cli keygen new
   nil_cli wallet new
   ```
4. Update the `.env` file with the private key and wallet address.

**Note:** If you don't specify a wallet address, the Hardhat plugin can create a default wallet linked to the private key. Therefore, only the private key is required.

## 🎯 Usage
To run all tests, use the following command:
```bash
npm run tests
```
Make sure to configure `.env` with the RPC URL and PRIVATE_KEY.

## 💪 Contributing
Contributions are always welcome! Please feel free to submit pull requests or open issues to discuss potential changes or improvements.

## 🚧 Work in Progress
**Please Note:** This project is currently under active development. Not all features are fully implemented, and you may encounter issues. If something isn't working as expected, don't hesitate to open an issue on our GitHub repository. We prioritize addressing these concerns and will get back to you promptly. Your feedback helps us improve!

Thank you for supporting our development efforts!
