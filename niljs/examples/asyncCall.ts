import { HttpTransport, PublicClient, generateSmartAccount } from "../src/index.js";
import { FAUCET_ENDPOINT, RPC_ENDPOINT, generateRandomAddress } from "./helpers.js";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: RPC_ENDPOINT,
  }),
  shardId: 1,
});

const smartAccount = await generateSmartAccount({
  shardId: 1,
  rpcEndpoint: RPC_ENDPOINT,
  faucetEndpoint: FAUCET_ENDPOINT,
});
const smartAccountAddress = smartAccount.address;

console.log("smartAccountAddress", smartAccountAddress);

const anotherAddress = generateRandomAddress();

const gasPrice = await client.getGasPrice(1);
const transaction = await smartAccount.sendTransaction({
  to: anotherAddress,
  value: 10_000_000n,
  feeCredit: 100_000n * gasPrice,
});

await transaction.wait();

const balance = await client.getBalance(anotherAddress, "latest");

console.log("balance", balance);

console.log("Transaction sent successfully");
