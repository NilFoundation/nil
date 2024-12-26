import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
} from "@nilfoundation/niljs";
import type { Address } from "abitype";

export function createClient(
  url: string | undefined = undefined,
): PublicClient {
  if (!process.env.NIL_RPC_ENDPOINT) {
    throw new Error("NIL_RPC_ENDPOINT should not be null");
  }
  return new PublicClient({
    transport: new HttpTransport({
      endpoint: url || process.env.NIL_RPC_ENDPOINT,
    }),
    shardId: 1,
  });
}

export async function createWallet(
  config: CreateWalletConfig = {},
  client: PublicClient = createClient(),
): Promise<WalletV1> {
  if (!process.env.PRIVATE_KEY) {
    throw new Error("PRIVATE_KEY should not be null");
  }
  const privateKey = process.env.PRIVATE_KEY.startsWith("0x")
    ? (process.env.PRIVATE_KEY as Address)
    : (`0x${process.env.PRIVATE_KEY}` as Address);
  const signer = new LocalECDSAKeySigner({
    privateKey: privateKey,
  });
  const walletAddress =
    config.address || (process.env.WALLET_ADDR as `0x${string}` | undefined);

  const wallet = new WalletV1({
    pubkey: signer.getPublicKey(),
    client: client,
    signer: signer,
    ...(walletAddress
      ? { address: walletAddress }
      : {
          salt: config.salt ?? BigInt(Math.round(Math.random() * 1000000)),
          shardId: config.shardId ?? 1,
        }),
  });

  if (config.faucetDeposit) {
    const faucet = new Faucet(client);
    console.log("Faucet depositing to wallet", wallet.address);
    await faucet.withdrawToWithRetry(wallet.address);
    const deployed = await wallet.checkDeploymentStatus();
    if (!deployed) {
      console.log("Deploying wallet", wallet.address);
      await wallet.selfDeploy();
    }
  }

  return wallet;
}

export type CreateWalletConfig = {
  address?: Address | Uint8Array;
  salt?: Uint8Array | bigint;
  shardId?: number;
  faucetDeposit?: boolean;
};
