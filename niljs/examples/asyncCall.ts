import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  bytesToHex,
  generateRandomPrivateKey,
  waitTillCompleted,
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

await faucet.withdrawToWithRetry(walletAddress, 100_000_000n);

await wallet.selfDeploy(true);

// biome-ignore lint/nursery/noConsole: <explanation>
console.log("Wallet deployed successfully");

const anotherAddress = WalletV1.calculateWalletAddress({
  pubKey: pubkey,
  shardId: 2,
  salt: 200n,
});

const hash = await wallet.sendMessage({
  to: anotherAddress,
  value: 10_000_000n,
  feeCredit: 100_000n,
});

await waitTillCompleted(client, 1, hash);

const balance = await client.getBalance(bytesToHex(anotherAddress), "latest");
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("balance", balance);

// biome-ignore lint/nursery/noConsole: <explanation>
console.log("Message sent successfully");
