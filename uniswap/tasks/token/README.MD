
# 🌐 Working with Tokens

---

## Overview

A **Token** is any contract that extends `NilTokenBase`. We have developed a simple example of a token contract, which you can find [here](https://github.com/NilFoundation/uniswap-v2-nil/blob/main/contracts/Token.sol). This contract represents the minimal implementation needed to create a token.

```solidity
contract Token is NilTokenBase {

    constructor(string memory _tokenName) payable {
        // Revert if the token name is an empty string
        require(bytes(_tokenName).length > 0, "Token name must not be empty");

        tokenName = _tokenName;
    }

    receive() external payable {}
}
```

Token owners can add any additional functionality they want on top of the built-in capabilities provided by `NilTokenBase`.

---

## 💡 How to Use

### 1. Deploy a Token Contract

To deploy your own token contract, use the following command:

```bash
npx hardhat ignition deploy ./ignition/modules/Token.ts --parameters ./ignition/parameters.json
```

Make sure to set the `tokenName` and `public key` in your `./ignition/parameters.json` file before deploying. Once the deployment is complete, you will receive the contract address in the response.

### 2. Fetch Token Data

After deploying the token contract, fetch the token information (name, ID, Balance) by using the following command:

```bash
npx hardhat token_info --address <Token Address>
```

Replace `<Token Address>` with the actual deployed contract address

### Mint and Send Token to a Smart Account

To mint token from two contracts and send it to a specified smart account, use the `mint-smart-account` task:

```bash
npx hardhat mint-smart-account --token0 <Token0 Address> --token1 <Token1 Address> --smart-account <Smart Account Address> --amount <Amount>
```

This will mint the specified amount of both tokens and send it to the specified smart account.

---
