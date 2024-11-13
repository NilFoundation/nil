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
import type { HandlerContext } from "./context";
import { shardNumber } from "./utils/conversion";
import { ensure0xPrefix } from "./utils/hex";

export function isStringArray(accounts: HttpNetworkAccountsConfig): accounts is string[] {
  return Array.isArray(accounts);
}

// Type guard to check if a network configuration is HTTP based
export function isHttpNetworkConfig(config: NetworkConfig): config is HttpNetworkConfig {
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
    throw new Error(`${networkName} is not an HTTP network config or is misconfigured.`);
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
  const pubKey = signer.getPublicKey();

  const newWalletSalt = new Uint8Array(32);
  const walletAddress = hre.config.walletAddress
    ? ensure0xPrefix(hre.config.walletAddress)
    : undefined;
  if (!walletAddress) {
    throw new Error(`Wallet address is not valid. Run 'npx hardhat wallet update' to fix it`);
  }

  // Set up network components
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint: url }),
    shardId: shardNumber(walletAddress),
  });
  const walletCode = await client.getCode(walletAddress, "latest");
  if (walletCode.length === 0) {
    throw new Error(
      `Wallet at address ${walletAddress} does not exist. Run 'npx hardhat wallet update' to fix it`,
    );
  }

  const faucet = new Faucet(client);

  const config: WalletV1Config = {
    pubkey: pubKey,
    client,
    signer,
    address: walletAddress,
  };
  const wallet = new WalletV1(config);

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
    directTxValue: hre.config.directTxValue ? BigInt(hre.config.directTxValue) : undefined,
    directTxFeeCredit: hre.config.directTxGasLimit
      ? BigInt(hre.config.directTxGasLimit)
      : undefined,
    debug: hre.config.debug ?? false,
  };
}
