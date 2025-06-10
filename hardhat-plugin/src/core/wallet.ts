import {
  type FaucetClient,
  LocalECDSAKeySigner,
  type PublicClient,
  SmartAccountV1,
  generateRandomPrivateKey,
  topUp,
} from "@nilfoundation/niljs";
import type { HttpNetworkConfig } from "hardhat/src/types/config";
import type { HardhatRuntimeEnvironment } from "hardhat/types";
import type { Address } from "viem";
import type { CreateSmartAccountConfig } from "../types";

export async function deployWallet(
  signer: LocalECDSAKeySigner,
  address: Address,
  client: PublicClient,
  faucetClient?: FaucetClient,
): Promise<SmartAccountV1> {
  const smartAccount = new SmartAccountV1({
    pubkey: signer.getPublicKey(),
    address: address,
    client: client,
    signer,
  });

  const deployed = await smartAccount.checkDeploymentStatus();
  if (faucetClient && !deployed) {
    console.log("Deploying smartAccount", smartAccount.address);
    await smartAccount.selfDeploy(faucetClient);
  }
  return smartAccount;
}

export async function createSmartAccount(
  hre: HardhatRuntimeEnvironment,
  config: CreateSmartAccountConfig,
): Promise<SmartAccountV1> {
  const client = hre.nil.getPublicClient();
  const faucetClient = hre.nil.getFaucetClient();
  const pk = generateRandomPrivateKey();
  const signer = new LocalECDSAKeySigner({
    privateKey: pk,
  });
  const smartAccount = new SmartAccountV1({
    pubkey: signer.getPublicKey(),
    client: client,
    signer,
    shardId: hre.config.defaultShardId ?? 1,
    salt: BigInt(Math.round(Math.random() * 1000000)),
  });

  if (config.topUp !== false) {
    await topUpSmartAccount(smartAccount.address, (hre.network.config as HttpNetworkConfig).url);
  }

  await smartAccount.selfDeploy(faucetClient);
  console.log("SmartAccount PK:", pk);
  return smartAccount;
}

export async function topUpSmartAccount(address: Address, rpc: string) {
  await topUp({
    address: address,
    faucetEndpoint: rpc,
    rpcEndpoint: rpc,
  });
}
