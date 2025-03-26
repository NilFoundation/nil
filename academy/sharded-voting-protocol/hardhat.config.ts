import "@nomicfoundation/hardhat-chai-matchers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@nomicfoundation/hardhat-ethers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@typechain/hardhat";
import * as dotenv from "dotenv";
import type { HardhatUserConfig } from "hardhat/config";

import "./task/run-lending-protocol";

dotenv.config();

const config: HardhatUserConfig = {
  ignition: {
    requiredConfirmations: 1,
  },
  defaultNetwork: "nil",
  solidity: {
    version: "0.8.28", // or your desired version
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
};

export default config;
