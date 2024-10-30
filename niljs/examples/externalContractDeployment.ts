import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  bytesToHex,
  convertEthToWei,
  externalDeploymentMessage,
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
const chainId = await client.chainId();

const deploymentMessage = externalDeploymentMessage(
  {
    salt: 100n,
    shard: 1,
    bytecode: WalletV1.code,
    abi: WalletV1.abi,
    args: [bytesToHex(pubkey)],
  },
  chainId,
);
const addr = bytesToHex(deploymentMessage.to);

console.log("walletAddress", addr);

const faucetHash = await faucet.withdrawTo(
  addr,
  convertEthToWei(0.1),
);

await waitTillCompleted(client, 1, bytesToHex(faucetHash));

await deploymentMessage.send(client);

while (true) {
  const code = await client.getCode(addr, "latest");
  if (code.length > 0) {
    console.log("code", bytesToHex(code));
    break;
  }
  await new Promise((resolve) => setTimeout(resolve, 1000));
}

console.log("Wallet deployed successfully");
