import { encodeFunctionData } from "viem";
import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  MINTER_ABI,
  MINTER_ADDRESS,
  PublicClient,
  WalletV1,
  bytesToHex,
  generateRandomPrivateKey,
  hexToBigInt,
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
await faucet.withdrawToWithRetry(walletAddress, 1_000_000_000_000n);

await wallet.selfDeploy(true);
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("Wallet deployed successfully");
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("walletAddress", walletAddress);

const hashMessage = await wallet.sendMessage({
  to: MINTER_ADDRESS,
  gas: 1_000_000n * 10n,
  value: 100_000_000n,
  data: encodeFunctionData({
    abi: MINTER_ABI,
    functionName: "create",
    args: [100_000_000n, walletAddress, "MY_TOKEN", walletAddress],
  }),
});

await waitTillCompleted(client, 1, hashMessage);

const n = hexToBigInt(walletAddress);

const tokens = await client.getCurrencies(walletAddress, "latest");
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("tokens", tokens);

const anotherAddress = WalletV1.calculateWalletAddress({
  pubKey: pubkey,
  shardId: 2,
  salt: 200n,
});

const sendHash = await wallet.sendMessage({
  to: anotherAddress,
  value: 10_000_000n,
  gas: 100_000n * 10n,
  tokens: [
    {
      id: n,
      amount: 100_00n,
    },
  ],
});

await waitTillCompleted(client, 1, sendHash);

const anotherTokens = await client.getCurrencies(
  bytesToHex(anotherAddress),
  "latest",
);

// biome-ignore lint/nursery/noConsole: <explanation>
console.log("anotherTokens", anotherTokens);
