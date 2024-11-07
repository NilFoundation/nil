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

const pubkey = signer.getPublicKey();

const wallet = new WalletV1({
  pubkey: pubkey,
  salt: 100n,
  shardId: 1,
  client,
  signer,
});
const walletAddress = wallet.address;

console.log("walletAddress", walletAddress);

const faucetHash = await faucet.withdrawTo(
  walletAddress,
  convertEthToWei(0.1),
);

await waitTillCompleted(client, bytesToHex(faucetHash));
await wallet.selfDeploy(true);

const code = await client.getCode(walletAddress, "latest");

console.log("code", bytesToHex(code));

console.log("Wallet deployed successfully");
