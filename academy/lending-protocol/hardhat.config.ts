import "@nomicfoundation/hardhat-chai-matchers";
import "@nomicfoundation/hardhat-ignition-ethers";
import "@nomicfoundation/hardhat-ethers";
import "@typechain/hardhat";
import "@nomicfoundation/hardhat-toolbox";
import "@nomiclabs/hardhat-ethers";


import * as dotenv from "dotenv";
import type { HardhatUserConfig } from "hardhat/config";

// Import niljs (correct scoped package)
import "@nilfoundation/niljs";

import "./task/run-lending-protocol";

dotenv.config(); // Load .env variables

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.28", // Match your Solidity version
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
    },
  },
};

export default config;