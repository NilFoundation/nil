
# 🌐 Working with Currencies

---

## Overview

A **Currency** is any contract that extends `NilCurrencyBase`. We have developed a simple example of a currency contract, which you can find [here](https://github.com/NilFoundation/uniswap-v2-nil/blob/main/contracts/Currency.sol). This contract represents the minimal implementation needed to create a currency.

```solidity
contract Currency is NilCurrencyBase {

    constructor(string memory _currencyName) payable {
        // Revert if the currency name is an empty string
        require(bytes(_currencyName).length > 0, "Currency name must not be empty");

        tokenName = _currencyName;
    }

    receive() external payable {}
}
```

Currency owners can add any additional functionality they want on top of the built-in capabilities provided by `NilCurrencyBase`.

---

## 💡 How to Use

### 1. Deploy a Currency Contract

To deploy your own currency contract, use the following command:

```bash
npx hardhat ignition deploy ./ignition/modules/Currency.ts --parameters ./ignition/parameters.json
```

Make sure to set the `currencyName` and `public key` in your `./ignition/parameters.json` file before deploying. Once the deployment is complete, you will receive the contract address in the response.

### 2. Fetch Currency Data

After deploying the currency contract, you can fetch the currency information (name, ID, Balance) by using the following command:

```bash
npx hardhat currency_info --address <Currency Address>
```

Replace `<Currency Address>` with the actual deployed contract address

### Mint and Send Currency to a Wallet

To mint currency from two contracts and send it to a specified wallet, use the mint-wallet task:

```bash
npx hardhat mint-wallet --currency0 <Currency0 Address> --currency1 <Currency1 Address> --wallet <Wallet Address> --amount <Amount>
```

This will mint the specified amount of both currencies and send it to the specified wallet

---