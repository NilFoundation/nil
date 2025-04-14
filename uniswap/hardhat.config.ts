import "@nomicfoundation/hardhat-chai-matchers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@nomicfoundation/hardhat-ethers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@nilfoundation/hardhat-nil-plugin";
import "@typechain/hardhat";
import * as dotenv from "dotenv";

// Basic
import "./tasks/basic/create-smart-account";

// Token Tasks
import "./tasks/token/info";
import "./tasks/token/mint-smart-account";
import "./tasks/token/deploy-token";

// Core Tasks
import "./tasks/uniswap/pair/get-reserves";
import "./tasks/uniswap/pair/mint";
import "./tasks/uniswap/pair/burn";
import "./tasks/uniswap/pair/swap";
import "./tasks/uniswap/factory/get-pair";
import "./tasks/uniswap/factory/create-pair";
import "./tasks/uniswap/factory/deploy-factory";

// Demo Tasks
import "./tasks/uniswap/demo-router";
import "./tasks/uniswap/demo-router-sync";
import { NilHardhatUserConfig } from "hardhat/types/config";

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
      nil: true,
      url: process.env.NIL_RPC_ENDPOINT,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
  },
  walletAddress: process.env.SMART_ACCOUNT_ADDR,
  defaultShardId: 1,
};

export default config;
