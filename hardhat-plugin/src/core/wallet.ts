import {
  type FaucetClient,
  LocalECDSAKeySigner,
  type PublicClient,
  SmartAccountV1,
  convertEthToWei,
  generateRandomPrivateKey,
} from "@nilfoundation/niljs";
import type { Address } from "viem";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { topUpSmartAccount } from "uniswap-v2-on-nil/tasks/basic/basic";

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

export async function createSmartAccount(hre: HardhatRuntimeEnvironment): Promise<SmartAccountV1> {
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

  await topUpSmartAccount(smartAccount.address)

  await smartAccount.selfDeploy(true);
  console.log("SmartAccount created successfully: " + smartAccount.address);
  return smartAccount;
}
