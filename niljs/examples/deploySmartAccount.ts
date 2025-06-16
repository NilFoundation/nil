import {
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
  bytesToHex,
  createSmartAccount,
  generateRandomPrivateKey,
} from "../src/index.js";
import { FAUCET_ENDPOINT, RPC_ENDPOINT } from "./helpers.js";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: RPC_ENDPOINT,
  }),
  shardId: 1,
});

const signer = new LocalECDSAKeySigner({
  privateKey: generateRandomPrivateKey(),
});

const pubkey = signer.getPublicKey();

const smartAccountAddress = await createSmartAccount({
  faucetEndpoint: FAUCET_ENDPOINT,
  shardId: 1,
  publicKey: pubkey,
  amount: 100_000_000_000_000_000_000_000n,
  salt: 100n,
});

console.log("smartAccountAddress", smartAccountAddress);

const smartAccount = new SmartAccountV1({
  address: smartAccountAddress,
  pubkey: pubkey,
  client,
  signer,
});

const code = await client.getCode(smartAccount.address, "latest");

console.log("code", bytesToHex(code));

console.log("Smart account deployed successfully");
