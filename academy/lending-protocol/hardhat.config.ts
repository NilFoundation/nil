import "@nomicfoundation/hardhat-chai-matchers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@nomicfoundation/hardhat-ethers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@typechain/hardhat";
import "@nomicfoundation/hardhat-toolbox";

import * as dotenv from "dotenv";
import type { HardhatUserConfig } from "hardhat/config";

import "./task/run-lending-protocol";

dotenv.config(); // Load .env variables

const config: HardhatUserConfig = {
  solidity: "0.8.28",
  networks: {
    nil: {
      url: process.env.NIL_RPC_ENDPOINT || "", // Ensure it's a string
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
  },
};

export default config;


