
# 🌐 Working with Pair

---

## Overview

A **Pair** is a contract that facilitates the swapping and liquidity management of two currencies. The `UniswapV2Pair` contract is responsible for handling the liquidity operations, including minting, burning, and swapping.

---

## 💡 How to Use

### 1. Retrieve Reserves

To fetch the reserves of the pair, use the following command:

```bash
npx hardhat get-reserves --pair <Pair Address>
```

Replace `<Pair Address>` with the actual pair address.

### 2. Mint Liquidity

To mint liquidity and add it to the pair, use the following command:

```bash
npx hardhat mint --pair <Pair Address> --wallet <User Wallet Address> --amount0 <Amount of Currency0> --amount1 <Amount of Currency1>
```

Replace `<Pair Address>`, `<User Wallet Address>`, `<Amount of Currency0>`, and `<Amount of Currency1>` with the actual values.

### 3. Swap Currencies

To swap currency0 for currency1, use the following command:

```bash
npx hardhat swap --pair <Pair Address> --wallet <User Wallet Address> --amount <Amount of Currency0>
```

Replace `<Pair Address>`, `<User Wallet Address>`, and `<Amount of Currency0>` with the actual values.

### 4. Burn Liquidity

To burn liquidity and withdraw your share of currencies from the pair, use the following command:

```bash
npx hardhat burn --pair <Pair Address> --wallet <User Wallet Address>
```

Replace `<Pair Address>` and `<User Wallet Address>` with the actual values.

---
