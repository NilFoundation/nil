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

console.log("walletAddress", walletAddress);

const faucetHash = await faucet.withdrawTo(
  walletAddress,
  convertEthToWei(0.1),
);

await waitTillCompleted(client, 1, bytesToHex(faucetHash));
await wallet.selfDeploy(true);

console.log("Wallet deployed successfully");

const anotherAddress = WalletV1.calculateWalletAddress({
  pubKey: pubkey,
  shardId: 1,
  salt: 200n,
});

await wallet.syncSendMessage({
  to: anotherAddress,
  value: 10n,
  gas: 100_000n * 10n,
});

while (true) {
  const balance = await client.getBalance(anotherAddress, "latest");
  if (balance > 0) {
    console.log("balance", balance);
    break;
  }
  await new Promise((resolve) => setTimeout(resolve, 1000));
}

console.log("Message sent successfully");
