import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  type WalletV1Config,
} from "@nilfoundation/niljs";
import type {
  HardhatRuntimeEnvironment,
  HttpNetworkAccountsConfig,
  HttpNetworkConfig,
  NetworkConfig,
} from "hardhat/types";
import { bytesToHex } from "viem";
import type { HandlerContext } from "./context";
import { shardNumber } from "./utils/conversion";
import { ensure0xPrefix } from "./utils/hex";

function isStringArray(
  accounts: HttpNetworkAccountsConfig,
): accounts is string[] {
  return Array.isArray(accounts);
}

// Type guard to check if a network configuration is HTTP based
function isHttpNetworkConfig(
  config: NetworkConfig,
): config is HttpNetworkConfig {
  return "url" in config;
}

// Function to setup the wallet and client
export async function setupWalletAndClient(
  hre: HardhatRuntimeEnvironment,
  originalRequest: any,
  originalSend: any,
): Promise<HandlerContext> {
  const networkName = "nil";
  const networkConfig = hre.config.networks[networkName];

  // Error handling if the network configuration is not HTTP or is missing
  if (!isHttpNetworkConfig(networkConfig)) {
    throw new Error(
      `${networkName} is not an HTTP network config or is misconfigured.`,
    );
  }

  // URL is guaranteed to exist and be a string here
  const url = networkConfig.url;
  if (!isStringArray(networkConfig.accounts)) {
    throw new Error("Accounts configuration is not an array of strings.");
  }

  const privateKey = ensure0xPrefix(networkConfig.accounts[0]);
  if (!privateKey) {
    throw new Error("No private key configured for the network.");
  }

  const signer = new LocalECDSAKeySigner({ privateKey });
  const pubKey = await signer.getPublicKey();

  const newWalletSalt = new Uint8Array(32);
  let walletAddress = hre.config.walletAddress
    ? ensure0xPrefix(hre.config.walletAddress)
    : undefined;
  if (!walletAddress) {
    walletAddress = bytesToHex(
      WalletV1.calculateWalletAddress({
        pubKey,
        shardId: 1,
        salt: newWalletSalt,
      }),
    );

    console.log(
      `Wallet address not found in configuration.\nGenerated wallet address for current private key: ${walletAddress}`,
    );
  }

  // Set up network components
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint: url }),
    shardId: shardNumber(walletAddress),
  });
  const faucet = new Faucet(client);

  const config: WalletV1Config = {
    pubkey: pubKey,
    client,
    signer,
    address: walletAddress,
  };
  const wallet = new WalletV1(config);

  const existingWallet = await client.getCode(walletAddress, "latest");
  if (existingWallet.length === 0) {
    console.log("Deploying new wallet...");

    wallet.salt = newWalletSalt;

    await faucet.withdrawToWithRetry(walletAddress, 1_000_000_000n);
    await wallet.selfDeploy();

    console.log("Deployed new wallet.");
  }

  return {
    hre,
    client,
    wallet,
    faucet,
    signer,
    originalSend,
    originalRequest,
    isRequest: false,
    feeCredit: BigInt(hre.config.feeCredit ?? 5_000_000),
    directTxValue: hre.config.directTxValue
      ? BigInt(hre.config.directTxValue)
      : undefined,
    directTxFeeCredit: hre.config.directTxGasLimit
      ? BigInt(hre.config.directTxGasLimit)
      : undefined,
    debug: hre.config.debug ?? false,
  };
}
