# Nil L1-Contracts

This smart-contracts module contains contracts required for communication with L1.
Solidity smart contracts in the repository are used for proof verification and state root updates of L2 on the L1 chain

1. NilRollup
2. NilVerifier

## NilRollup

- NilRollup contract is the entrypoint contract for batch commits and proof verifications initiated by syncCommittee

- NilRollup contract contains 2 main functions:
   1. [commitBatch](./contracts/NilRollup.sol#L284)
   2. [updateState](./contracts/NilRollup.sol#L322)

## NilVerifier

1. [NilVerifier-Contract](./contracts/verifier/NilVerifier.sol) contains the [verify](./contracts/verifier/NilVerifier.sol#L9) proof logic
2. NilVerifier contract is non-upgradeable and stateless contract


## Contract directory-structure

```
.
├── README.md
├── codecov.yml
├── contracts
│   ├── NilAccessControl.sol
│   ├── NilRollup.sol
│   ├── interfaces
│   │   ├── INilAccessControl.sol
│   │   ├── INilRollup.sol
│   │   └── INilVerifier.sol
│   └── verifier
│       └── NilVerifier.sol
├── deploy
│   ├── config
│   │   ├── archive
│   │   │   └── nil-deployment-config-archive.json
│   │   ├── config-helper.ts
│   │   ├── nil-deployment-config.json
│   │   └── nil-types.ts
│   ├── deploy-contracts.ts
│   ├── deploy-nil-verifier.ts
│   ├── deploy-nilrollup.ts
│   └── upgrade-nilrollup.ts
├── foundry.toml
├── hardhat.config.ts
├── package.json
├── remappings.txt
├── scripts
│   ├── access-control
│   │   ├── admin
│   │   │   ├── get-all-admins.ts
│   │   │   ├── grant-admin-access.ts
│   │   │   ├── is-an-admin.ts
│   │   │   └── revoke-admin-access.ts
│   │   ├── get-role-members.ts
│   │   ├── has-a-role.ts
│   │   ├── owner
│   │   │   ├── accept-ownership.ts
│   │   │   ├── get-owner.ts
│   │   │   ├── get-pending-owner.ts
│   │   │   ├── has-ownership-role.ts
│   │   │   └── transfer-ownership.ts
│   │   └── proposer
│   │       ├── get-all-proposer-admins.ts
│   │       ├── get-all-proposers.ts
│   │       ├── grant-proposer-access.ts
│   │       ├── grant-proposer-admin-access.ts
│   │       ├── is-a-proposer.ts
│   │       ├── renounce-proposer-access.ts
│   │       ├── revoke-proposer-access.ts
│   │       └── revoke-proposer-admin-access.ts
│   ├── geth-ops
│   │   └── fund-wallet.ts
│   ├── proxy
│   │   ├── query-proxy-admin.ts
│   │   └── transfer-proxyadmin-ownership.ts
│   ├── utils
│   │   └── roles.ts
│   ├── verify
│   │   ├── verify-nil-verifier.ts
│   │   └── verify-nilrollup.ts
│   └── wallet
│       ├── create-wallet-with-funding.ts
│       └── fund-wallet.ts
├── setup.sh
├── slither.config.json
├── test
│   ├── BaseTest.sol
│   ├── NilRollup.t.sol
│   ├── NilRollupAccessControl.t.sol
│   ├── config
│   │   ├── blob-data-input.json
│   │   └── update-state-invalid-scenarios.json
│   ├── misc
│   │   ├── CustomTransparentUpgradeableProxy.sol
│   │   └── EmptyContract.sol
│   └── mocks
│       ├── NilRollupMockBlob.sol
│       └── NilRollupMockBlobInvalidScenario.sol
└── tsconfig.json
```

## Dependencies

### Node.js

First install [`Node.js`](https://nodejs.org/en) and [`npm`](https://www.npmjs.com/).
Run the following command to install [`yarn`](https://classic.yarnpkg.com/en/):

```bash
npm install --global yarn
```

### Foundry

Install `foundryup`, the Foundry toolchain installer:

```bash
curl -L https://foundry.paradigm.xyz | bash
```

If you do not want to use the redirect, feel free to manually download the `foundryup` installation script from [here](https://raw.githubusercontent.com/foundry-rs/foundry/master/foundryup/foundryup).

Then, run `foundryup` in a new terminal session or after reloading `PATH`.

Other ways to install Foundry can be found [here](https://github.com/foundry-rs/foundry#installation).

### Hardhat

Run the following command to install [Hardhat](https://hardhat.org/) and other dependencies.

```
yarn install
```

## Build

1. run the setup script to:
  - download all node dependencies
  - download the git submodules
  - hardhat clean & compile
  - forge build, clean and compile

```sh
npm run setup
```

2. copy `.env.example` to `.env`

3. set all pre-requisite variables in .env
   - WALLET_ADDRESS
   - PRIVATE_KEY

- This address is same as the address used for deployment and acts as the owner of the NilChain contract
- The address is to be used when running SyncCommitee node


## Local Run

- For build pipeline or local testing, the contract is to be deployed on local Nil Node

### Please follow the steps mentioned below:

1. copy `.env.example` to `.env`
2. set all pre-requisite variables in .env

```
GETH_RPC_ENDPOINT="http://localhost:8545"
GETH_PRIVATE_KEY=""
GETH_WALLET_ADDRESS=""

SEPOLIA_RPC_ENDPOINT="https://1rpc.io/sepolia"
SEPOLIA_WALLET_ADDRESS=""
SEPOLIA_PRIVATE_KEY=""
```

3. This address is same as the address used for deployment and acts as the owner of the NilRollup contract

- Compilation:

   - hardhat compilation:

   ```shell
   npx hardhat compile
   ```

   - foundry compilation

   ```shell
   forge compile
   ```

   - foundry test

   ```shell
   forge test
   ```


## Deployment Steps:

### Deploy NilContracts on geth instance

```shell
npx hardhat deploy --network geth --tags NilContracts
```

### Deploy NilContracts on sepolia instance

- script deploys and verifies the deployed contract on sepolia network

```shell
npx hardhat deploy --network sepolia --tags NilContracts
```
