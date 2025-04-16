import {
  type FaucetClient,
  LocalECDSAKeySigner,
  type PublicClient,
  SmartAccountV1,
  convertEthToWei,
  generateRandomPrivateKey,
  topUp,
} from "@nilfoundation/niljs";
import type { Address } from "viem";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { CreateSmartAccountConfig } from "../types";
import { HttpNetworkConfig } from "hardhat/src/types/config";

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

  if (faucetClient) {
    const faucets = await faucetClient.getAllFaucets();
    await faucetClient.topUpAndWaitUntilCompletion(
      {
        amount: convertEthToWei(1),
        smartAccountAddress: address,
        faucetAddress: faucets.NIL,
      },
      client,
    );
    console.log("Faucet depositing to smart account", smartAccount.address);
  }

  const deployed = await smartAccount.checkDeploymentStatus();
  if (!deployed) {
    console.log("Deploying smartAccount", smartAccount.address);
    await smartAccount.selfDeploy(true);
  }
  return smartAccount;
}

export async function createSmartAccount(
  hre: HardhatRuntimeEnvironment,
  config: CreateSmartAccountConfig,
): Promise<SmartAccountV1> {
  const client = hre.nil.getPublicClient();
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

  await smartAccount.selfDeploy(true);
  console.log("SmartAccount created successfully: " + smartAccount.address);
  return smartAccount;
}

export async function topUpSmartAccount(
  address: Address,
  rpc: string,
) {
  await topUp({
    address: address,
    faucetEndpoint: rpc,
    rpcEndpoint: rpc
  });
}
