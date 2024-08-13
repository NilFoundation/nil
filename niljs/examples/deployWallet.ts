import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  bytesToHex,
  generateRandomPrivateKey,
} from "../src";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: "http://127.0.0.1:8529",
  }),
  shardId: 1,
});

const faucet = new Faucet(client);

const signer = new LocalECDSAKeySigner({
  privateKey: generateRandomPrivateKey(),
});

const pubkey = await signer.getPublicKey();

const wallet = new WalletV1({
  pubkey: pubkey,
  salt: 100n,
  shardId: 1,
  client,
  signer,
});
const walletAddress = await wallet.getAddressHex();

// biome-ignore lint/nursery/noConsole: <explanation>
console.log("walletAddress", walletAddress);

await faucet.withdrawToWithRetry(walletAddress, 100000n);
await wallet.selfDeploy(true);

const code = await client.getCode(walletAddress, "latest");
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("code", bytesToHex(code));
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("Wallet deployed successfully");
