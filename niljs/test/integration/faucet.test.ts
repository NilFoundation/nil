import {
  Faucet,
  FaucetClient,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  convertEthToWei,
  generateRandomPrivateKey,
} from "../../src/index.js";
import { testEnv } from "../testEnv.js";

const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: testEnv.endpoint,
  }),
  shardId: 1,
});

const faucetClient = new FaucetClient({
  transport: new HttpTransport({
    endpoint: testEnv.faucetServiceEndpoint,
  }),
  shardId: 1,
});

test("getAllFaucets", async () => {
  const faucets = await faucetClient.getAllFaucets();

  expect(Object.keys(faucets).length).toBeGreaterThan(0);
});

test("mint tokens", async () => {
  const legacyFaucet = new Faucet(client);
  const faucets = await faucetClient.getAllFaucets();

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

  const faucet = Object.values(faucets)[0];
  const walletAddress = wallet.address;
  await legacyFaucet.withdrawToWithRetry(walletAddress, convertEthToWei(1));
  await wallet.selfDeploy(true);

  const tx = await faucetClient.topUpAndWaitUntilCompletion(
    {
      faucetAddress: faucet,
      walletAddress,
      amount: 10,
    },
    client,
  );

  const tokens = await client.getCurrencies(walletAddress, "latest");

  expect(tokens).toBeDefined();
  expect(Object.keys(tokens).length).toBeGreaterThan(0);
  expect(tokens[faucet]).toBeDefined();
});
