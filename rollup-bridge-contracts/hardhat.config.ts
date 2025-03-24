import * as dotenv from "dotenv";
import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";
import "hardhat-preprocessor";
import "hardhat-deploy";
require('@openzeppelin/hardhat-upgrades');
import { resolve } from "path";
import fs from "fs";
dotenv.config();


const config: HardhatUserConfig = {
  ignition: {
    requiredConfirmations: 1,
  },
  // defaultNetwork: "nil",
  solidity: {
    version: "0.8.28",
    settings: {
      viaIR: true,
      optimizer: {
        enabled: true,
        runs: 200,
      },
      evmVersion: "cancun",
    },
  },
  paths: {
    sources: "./contracts",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts",
  },
  etherscan: {
    apiKey: {
      mainnet: process.env.ETHERSCAN_API_KEY || "",
      sepolia: process.env.ETHERSCAN_API_KEY || "",
    },
  },
  networks: {
    anvil: {
      chainId: 31337,
      url: process.env.ANVIL_RPC_ENDPOINT,
      accounts: process.env.ANVIL_PRIVATE_KEY ? [process.env.ANVIL_PRIVATE_KEY] : [],
    },
    geth: {
      chainId: 1337,
      url: process.env.GETH_RPC_ENDPOINT,
      accounts: process.env.GETH_PRIVATE_KEY ? [process.env.GETH_PRIVATE_KEY] : [],
    },
    sepolia: {
      chainId: 11155111,
      url: process.env.SEPOLIA_RPC_ENDPOINT,
      accounts: process.env.SEPOLIA_PRIVATE_KEY ? [process.env.SEPOLIA_PRIVATE_KEY] : [],
      gas: 1000000
    }
  },
  namedAccounts: {
    deployer: {
      default: 0,
    },
  },
  typechain: {
    outDir: "./typechain",
    target: "ethers-v6",
  },
  gasReporter: {
    enabled: process.env.REPORT_GAS !== undefined,
    excludeContracts: ["src/test"],
    currency: "USD",
  },


  mocha: {
    timeout: 10000000,
  },
};

export default config;
