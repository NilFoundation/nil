
# 🌐 Working with Factory

---

## Overview

A **Factory** is a contract that creates and manages pairs of currencies. The `UniswapV2Factory` contract is responsible for deploying new pairs and retrieving existing pair addresses.

---

## 💡 How to Use

### 1. Deploy the Factory Contract

To deploy the factory contract, use the following command:

```bash
npx hardhat ignition deploy ./ignition/modules/Factory.ts --parameters ./ignition/parameters.json
```

### 2. Deploy a Pair

To deploy a new pair, use the following command:

```bash
npx hardhat create-pair --factory <Factory Address> --currency0 <Currency0 Address> --currency1 <Currency1 Address>
```

Replace `<Factory Address>`, `<Currency0 Address>`, and `<Currency1 Address>` with the actual addresses.

### 3. Retrieve Pair Address

To fetch the pair address for two currencies, use the following command:

```bash
npx hardhat get-pair --factory <Factory Address> --currency0 <Currency0 Address> --currency1 <Currency1 Address>
```

Replace `<Factory Address>`, `<Currency0 Address>`, and `<Currency1 Address>` with the actual addresses.

---