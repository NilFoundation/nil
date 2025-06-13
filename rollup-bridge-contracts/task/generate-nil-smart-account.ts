import { task } from "hardhat/config";
import { Wallet, ethers } from 'ethers';
import * as fs from "fs";
import {
  FaucetClient,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
  convertEthToWei,
  generateRandomPrivateKey,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import "dotenv/config";
import { decodeFunctionResult, encodeFunctionData } from "viem";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { generateNilSmartAccount } from "./nil-smart-account";

let deployerAccount: SmartAccountV1 | null = null;

// npx hardhat generate-nil-smart-account --networkname local
task("generate-nil-smart-account", "Deploys a SmartAccount on Nil Chain")
  .addParam("networkname", "The network to use") // Mandatory parameter
  .setAction(async (taskArgs) => {
    const networkName = taskArgs.networkname;
    console.log(`Running task on network: ${networkName}`);

    [deployerAccount] = await generateNilSmartAccount(networkName);
    if (!deployerAccount) throw new Error("SmartAccount is not initialized.");
  });
