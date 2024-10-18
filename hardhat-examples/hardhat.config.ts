import "@nomicfoundation/hardhat-ethers";
import "@nomicfoundation/hardhat-toolbox";
import * as dotenv from "dotenv";
import "@nilfoundation/hardhat-plugin";
import type { NilHardhatUserConfig } from "@nilfoundation/hardhat-plugin";

dotenv.config();

const config: NilHardhatUserConfig = {
  solidity: "0.8.26",
  defaultNetwork: "nil",
  ignition: {
    requiredConfirmations: 1,
  },
  networks: {
    nil: {
      url: process.env.NIL_RPC_ENDPOINT,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
  },
  walletAddress: process.env.WALLET_ADDR,
  debug: true,
};
export default config;
