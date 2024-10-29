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
const walletAddress = wallet.address;
const faucetHash = await faucet.withdrawTo(
  walletAddress,
  convertEthToWei(0.1),
);

await waitTillCompleted(client, 1, bytesToHex(faucetHash));

await wallet.selfDeploy(true);

console.log("Wallet deployed successfully");
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

const tokens = await client.getCurrencies(walletAddress, "latest");

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
      id: walletAddress,
      amount: 100_00n,
    },
  ],
});

await waitTillCompleted(client, 1, sendHash);

const anotherTokens = await client.getCurrencies(
  anotherAddress,
  "latest",
);

console.log("anotherTokens", anotherTokens);
