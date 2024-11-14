import "@nomicfoundation/hardhat-ethers";
import "@nomicfoundation/hardhat-toolbox";
import type { NilHardhatUserConfig } from "../../../src";
import { getValue } from "../../../src/config";
import "../../../src";
import * as dotenv from "dotenv";

dotenv.config();

const walletAddress = getValue("address");
const privateKey = getValue("private_key");
const rpcEndpoint = getValue("rpc_endpoint");

const config: NilHardhatUserConfig = {
  solidity: {
    version: "0.8.21",
    settings: {
      metadata: {
        appendCBOR: false,
      },
    },
  },
  networks: {
    nil: {
      url: rpcEndpoint,
      accounts: privateKey ? [privateKey] : [],
    },
  },
  defaultNetwork: "nil",
  walletAddress: walletAddress,
};
export default config;
