import {
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
} from "@nilfoundation/niljs";
import type { Address } from "abitype";

export async function createClient(): Promise<{
  wallet: WalletV1;
  publicClient: PublicClient;
  signer: LocalECDSAKeySigner;
}> {
  const walletAddress = process.env.WALLET_ADDR as Address | undefined;

  if (!walletAddress) {
    throw new Error("WALLET_ADDR is not set in environment variables");
  }

  const endpoint = process.env.NIL_RPC_ENDPOINT;

  if (!endpoint) {
    throw new Error("NIL_RPC_ENDPOINT is not set in environment variables");
  }

  const publicClient = new PublicClient({
    transport: new HttpTransport({
      endpoint: endpoint,
    }),
    shardId: 1,
  });

  const signer = new LocalECDSAKeySigner({
    privateKey: `0x${process.env.PRIVATE_KEY}`,
  });
  const pubkey = await signer.getPublicKey();

  const wallet = new WalletV1({
    pubkey: pubkey,
    address: walletAddress,
    client: publicClient,
    signer,
  });

  return { wallet, publicClient, signer };
}
