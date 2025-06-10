import {
  FaucetClient,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
  bytesToHex,
  generateRandomPrivateKey,
} from "../src/index.js";
import { FAUCET_ENDPOINT, RPC_ENDPOINT } from "./helpers.js";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: RPC_ENDPOINT,
  }),
  shardId: 1,
});

const faucetClinet = new FaucetClient({
  transport: new HttpTransport({
    endpoint: FAUCET_ENDPOINT,
  }),
});

const signer = new LocalECDSAKeySigner({
  privateKey: generateRandomPrivateKey(),
});

const pubkey = signer.getPublicKey();

const smartAccount = new SmartAccountV1({
  pubkey: pubkey,
  salt: 100n,
  shardId: 1,
  client,
  signer,
});
const smartAccountAddress = smartAccount.address;

console.log("smartAccountAddress", smartAccountAddress);

await smartAccount.selfDeploy(faucetClinet);

const code = await client.getCode(smartAccountAddress, "latest");

console.log("code", bytesToHex(code));

console.log("Smart account deployed successfully");
