<h1 align="center">@nilfoundation/smart-contracts</h1>

<br />

<p align="center">
  An NPM package housing Solidity extension libraries for working with =nil;.
</p>

<br />

## Table of Contents
- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
- [License](#license)

## Overview

This NPM package contains the Solidity libraries for interacting with the =nil; cluster. These extensions provide access to essential functionalities of =nil; such as [making async calls](https://docs.nil.foundation/nil/getting-started/essentials/handling-async-execution), [accepting external messages](https://docs.nil.foundation/nil/getting-started/essentials/receiving-ext-transactions) and [working with custom tokens](https://docs.nil.foundation/nil/getting-started/essentials/tokens).

## Installation

To install the package:

```bash
npm install @nilfoundation/smart-contracts
```

## Contracts

The package includes four contracts:

* [`Faucet.sol`](./contracts/Faucet.sol) is a service contract for distributing tokens
* [`Nil.sol`](./contracts/Nil.sol) is the extension library that allows for making async calls and performing other important operations
* [`NilTokenBase.sol`](./contracts/NilTokenBase.sol) is the base contract for custom tokens on the cluster
* [`SmartAccount.sol`](./contracts/SmartAccount.sol) is the default smart account that is deployed by the =nil; CLI and `Nil.js`

## Usage

To use the package, simply import it in a JS/TS or Solidity project: 

```typescript
import SmartAccount_compiled from '@nilfoundation/smart-contracts/artifacts/SmartAccount.json';
const smartAccount_bytecode = `0x${SmartAccount_compiled.evm.bytecode.object}`
```

```solidity
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/SmartAccount.sol";
```

## License

[MIT](./LICENSE)


