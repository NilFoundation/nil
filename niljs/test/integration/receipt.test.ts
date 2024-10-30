import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  convertEthToWei,
  generateRandomPrivateKey,
  waitTillCompleted,
} from "../../src/index.js";
import { testEnv } from "../../test/testEnv.js";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: testEnv.endpoint,
  }),
  shardId: 1,
});

test("Receipt test", async ({ expect }) => {
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

  expect(walletAddress).toBeDefined();

  const faucetHash = await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(0.1));

  const receipts = await waitTillCompleted(client, 1, faucetHash);

  expect(receipts).toBeDefined();
  for (const receipt of receipts) {
    expect(receipt).toBeDefined();
    expect(receipt.gasPrice).toBeDefined();
    expect(receipt.gasUsed).toBeDefined();
    expect(receipt.gasPrice).toBeTypeOf("bigint");
  }
});
