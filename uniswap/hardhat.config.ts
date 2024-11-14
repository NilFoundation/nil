import "@nomicfoundation/hardhat-chai-matchers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@nomicfoundation/hardhat-ethers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@typechain/hardhat";
import "@nilfoundation/hardhat-plugin";
import type { NilHardhatUserConfig } from "@nilfoundation/hardhat-plugin";
import * as dotenv from "dotenv";

// Currency Tasks
import "./tasks/currency/info";
import "./tasks/currency/mint-wallet";

// Core Tasks
import "./tasks/core/pair/get-reserves";
import "./tasks/core/pair/mint";
import "./tasks/core/pair/burn";
import "./tasks/core/pair/swap";
import "./tasks/core/factory/get-pair";
import "./tasks/core/factory/create-pair";

// Demo Tasks
import "./tasks/core/demo";
import "./tasks/core/demo-router";
import "./tasks/core/demo-router-sync";

dotenv.config();

const config: NilHardhatUserConfig = {
  ignition: {
    requiredConfirmations: 1,
  },
  defaultNetwork: "nil",
  solidity: {
    version: "0.8.26", // or your desired version
    settings: {
      viaIR: true, // needed to compile router
      optimizer: {
        enabled: true,
        runs: 200,
      },
    },
  },
  networks: {
    nil: {
      url: process.env.NIL_RPC_ENDPOINT,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
  },
  walletAddress: process.env.WALLET_ADDR,
};

export default config;
