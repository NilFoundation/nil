import {FaucetClient, LocalECDSAKeySigner, PublicClient, SmartAccountV1, topUp} from "@nilfoundation/niljs";
import {Address} from "viem";

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
        amount: 1_000_000_000_000_000_000n,
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
    await smartAccount.selfDeploy();
  }
  return smartAccount
}
