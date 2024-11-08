import { encodeFunctionData } from "viem";
import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
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

const pubkey = signer.getPublicKey();

const wallet = new WalletV1({
  pubkey: pubkey,
  salt: 100n,
  shardId: 1,
  client,
  signer,
});
const walletAddress = wallet.address;

const anotherWallet = new WalletV1({
  pubkey: pubkey,
  salt: 200n,
  shardId: 1,
  client,
  signer,
});

console.log("walletAddress", walletAddress);

console.log("anotherWallet", anotherWallet.address);

await faucet.withdrawToWithRetry(walletAddress, 100_000_000n);
await faucet.withdrawToWithRetry(anotherWallet.address, 100_000_000n);

await wallet.selfDeploy(true);
await anotherWallet.selfDeploy(true);

console.log("Wallet deployed successfully");

const bounceAddress = WalletV1.calculateWalletAddress({
  pubKey: pubkey,
  shardId: 1,
  salt: 300n,
});

// bounce message
const hash = await wallet.sendMessage({
  to: anotherWallet.address,
  value: 10_000_000n,
  bounceTo: bounceAddress,
  feeCredit: 100_000n * 10n,
  data: encodeFunctionData({
    abi: WalletV1.abi,
    functionName: "syncCall",
    args: [walletAddress, 100_000, 10_000_000, "0x"],
  }),
});

await waitTillCompleted(client, hash);

console.log("bounce address", bounceAddress);

const balance = await client.getBalance(bounceAddress, "latest");

console.log("balance", balance);

console.log("Message sent successfully");
