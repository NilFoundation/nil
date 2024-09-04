<h1 align="center">@nilfoundation/smart-contracts</h1>

<br />

<p align="center">
  Smart-contracts implementations of the =nil; network.
</p>

<br />

## Table of Contents
- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
- [License](#license)
- [Acknowledgements](#acknowledgements)

## Overview
This package contains the smart-contracts implementations of the =nil; network. The smart-contracts are written in Solidity and are used to deploy and interact with the =nil; network.

## Installation
To install the package, run the following command:

```bash
npm install @nilfoundation/smart-contracts
```

## Usage
To use the package, import the smart-contracts in your project:

```typescript
import Wallet_compiled from '@nilfoundation/smart-contracts/artifacts/Wallet.json';
const wallet_bytecode = `0x${Wallet_compiled.evm.bytecode.object}`
```

```solidity
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Wallet.sol";
```

## License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgements
This project is supported by the [NIL Foundation](https://nil.foundation/).
