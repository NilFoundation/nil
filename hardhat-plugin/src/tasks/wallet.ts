import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  type WalletV1Config,
  generateRandomPrivateKey,
} from "@nilfoundation/niljs";
import { scope } from "hardhat/config";
import { types } from "hardhat/internal/core/config/config-env";
import type { HardhatRuntimeEnvironment } from "hardhat/types";
import type { Address } from "viem";
import * as config from "../config";
import { isHttpNetworkConfig, isStringArray } from "../setup";
import { shardNumber } from "../utils/conversion";
import { ensure0xPrefix } from "../utils/hex";

const walletTask = scope("wallet", "Wallet tasks");

walletTask
  .task("update", "Create a new wallet if the current one doesn't exist")
  .addOptionalParam(
    "force",
    "Create a new wallet even if the current one is valid",
    false,
    types.boolean,
  )
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    let walletAddress = config.getValue("address");
    const networkName = "nil";
    const networkConfig = hre.config.networks[networkName];

    // Error handling if the network configuration is not HTTP or is missing
    if (!isHttpNetworkConfig(networkConfig)) {
      throw new Error(`${networkName} is not an HTTP network config or is misconfigured.`);
    }

    const accounts = networkConfig.accounts as string[];

    if (accounts.length === 0) {
      const privateKey = generateRandomPrivateKey();
      accounts.push(privateKey);
    }

    if (!isStringArray(networkConfig.accounts)) {
      throw new Error("Accounts configuration is not an array of strings.");
    }

    let isWalletValid = false;
    if (walletAddress && !taskArgs.force) {
      const client = new PublicClient({
        transport: new HttpTransport({ endpoint: networkConfig.url }),
        shardId: shardNumber(walletAddress),
      });

      const walletCode = await client.getCode(walletAddress as Address, "latest");
      isWalletValid = walletCode.length !== 0;
    }

    if (!isWalletValid) {
      if (taskArgs.force) {
        console.log("Forcibly create a new wallet...");
      } else {
        console.log("Current wallet is invalid, creating a new one...");
      }
      const privateKey = ensure0xPrefix(networkConfig.accounts[0]);
      if (!privateKey) {
        throw new Error("No private key configured for the network.");
      }

      const signer = new LocalECDSAKeySigner({ privateKey });
      const pubKey = await signer.getPublicKey();

      const newWalletSalt = crypto.getRandomValues(new Uint8Array(32));

      walletAddress = WalletV1.calculateWalletAddress({ pubKey, shardId: 1, salt: newWalletSalt });

      const client = new PublicClient({
        transport: new HttpTransport({ endpoint: networkConfig.url }),
        shardId: shardNumber(walletAddress),
      });

      const walletConfig: WalletV1Config = {
        pubkey: pubKey,
        client,
        signer,
        address: ensure0xPrefix(walletAddress),
      };
      const wallet = new WalletV1(walletConfig);

      wallet.salt = newWalletSalt;
      const faucet = new Faucet(client);

      await faucet.withdrawToWithRetry(ensure0xPrefix(walletAddress), 1_000_000_000n);
      await wallet.selfDeploy();

      config.setValue("address", walletAddress);
      config.setValue("private_key", privateKey);
      hre.config.walletAddress = walletAddress;
      console.log(`New wallet created at ${walletAddress}`);
    } else {
      console.log("Wallet is already valid. Use '--force' option to create a new one.");
    }
  });

walletTask
  .task("info", "Print info about current wallet")
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    const walletAddress = config.getValue("address");
    const privateKey = config.getValue("private_key");
    console.log("Current wallet:");
    console.log(`  address: ${walletAddress}`);
    console.log(`  privateKey: ${privateKey}`);
    console.log("  balance: TODO");
  });
