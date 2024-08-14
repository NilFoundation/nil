import { encodeFunctionData } from "viem";
import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  bytesToHex,
  convertEthToWei,
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
const faucetHash = await faucet.withdrawTo(
  walletAddress,
  convertEthToWei(0.1),
);

await waitTillCompleted(client, 1, bytesToHex(faucetHash));

await wallet.selfDeploy(true);
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("Wallet deployed successfully");
// biome-ignore lint/nursery/noConsole: <explanation>
console.log("walletAddress", walletAddress);

const hashMessage = await wallet.sendMessage({
  to: walletAddress,
  feeCredit: 1_000_000n * 10n,
  value: 0n,
  data: encodeFunctionData({
    abi: WalletV1.abi,
    functionName: "setCurrencyName",
    args: ["MY_TOKEN"],
  }),
});

await waitTillCompleted(client, 1, hashMessage);

const hashMessage2 = await wallet.sendMessage({
  to: walletAddress,
  feeCredit: 1_000_000n * 10n,
  value: 0n,
  data: encodeFunctionData({
    abi: WalletV1.abi,
    functionName: "mintCurrency",
    args: [100_000_000n],
  }),
});

await waitTillCompleted(client, 1, hashMessage2);

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
  feeCredit: 100_000n * 10n,
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
