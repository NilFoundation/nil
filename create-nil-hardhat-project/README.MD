<h1 align="center">create-nil-hardhat-project</h1>

<br />

<p align="center">
  The TypeScript generator of Hardhat projects working on top of =nil;
</p>

## Table of contents

* [Overview](#-overview)
* [Installation](#-installation)
* [Configuration](#️-configuration)
* [Usage](#-usage)
* [Contributing](#-contributing)
* [Work in progress](#-work-in-progress)
* [License](#license)

## 🚀 Overview

This project houses `create-nil-hardhat-project`, a generator of Hardhat projects that work on top of =nil;. 


## 🔧 Installation

Clone the repository:

```bash
git clone https://github.com/NilFoundation/nil.git
cd ./nil/create-nil-hardhat-project
```
Install dependencies:

```bash
npm install
```

## ⚙️ Configuration

1. Create a `.env` file in the root directory based on the given `.env.example` file.
2. Update the `.env` file with the RPC URL. The default value corresponds to a locally running =nil; node.
3. Generate a private key and a new smart account using the =nil; CLI:

   ```bash
   nil keygen new
   nil smart-account new
   ```

4. Update the `.env` file with the private key and the smart account address.


## 🎯 Usage

To run all tests:

```bash
npm run tests
```

## 💪 Contributing

Contributions are always welcome! Feel free to submit pull requests or open issues to discuss potential changes or improvements.

## 🚧 Work in Progress

This project is currently under active development. Not all features are fully implemented and the project may behave unexpectedly in certain cases. If an issue arises when interacting with the project, open a new GitHub issue and detail the problem.

## License

[MIT](./LICENSE)
