import { PublicClient } from "../clients/PublicClient.js";
import { LocalECDSAKeySigner } from "../signers/LocalECDSAKeySigner.js";
import { generateRandomPrivateKey } from "../signers/privateKey.js";
import { SmartAccountV1 } from "../smart-accounts/SmartAccountV1/SmartAccountV1.js";
import { HttpTransport } from "../transport/HttpTransport.js";
import { createSmartAccount } from "./faucet.js";

export async function generateSmartAccount(options: {
  shardId: number;
  rpcEndpoint: string;
  faucetEndpoint: string;
}) {
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: options.rpcEndpoint,
    }),
    shardId: options.shardId,
  });

  const privateKey = generateRandomPrivateKey();
  const signer = new LocalECDSAKeySigner({
    privateKey: privateKey,
  });

  const salt = BigInt(Math.floor(Math.random() * 10000));

  const address = await createSmartAccount({
    faucetEndpoint: options.faucetEndpoint,
    shardId: options.shardId,
    publicKey: signer.getPublicKey(),
    salt: salt,
    amount: BigInt(100_000_000_000_000_000_000_000n),
  });

  const smartAccount = new SmartAccountV1({
    pubkey: signer.getPublicKey(),
    client: client,
    signer: signer,
    address: address,
  });
  return smartAccount;
}
