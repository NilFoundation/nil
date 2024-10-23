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
} from "../../src/index.js";
import { testEnv } from "../testEnv.js";
const client = new PublicClient({
  transport: new HttpTransport({
    endpoint: testEnv.endpoint,
  }),
  shardId: 1,
});

test("mint and transfer tokens", async () => {
  const faucet = new Faucet(client);

  const gasPriceOnShard1 = await client.getGasPrice(1);
  const gasPriceOnShard2 = await client.getGasPrice(2);

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
  await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(0.1));

  await wallet.selfDeploy(true);

  const mintCount = 100_000_000n;

  {
    const hashMessage = await wallet.setCurrencyName("MY_TOKEN");
    await waitTillCompleted(client, 1, hashMessage);
  }

  {
    const hashMessage = await wallet.mintCurrency(mintCount);
    await waitTillCompleted(client, 1, hashMessage);
  }

  const tokens = await client.getCurrencies(walletAddress, "latest");

  expect(tokens).toBeDefined();
  expect(Object.keys(tokens).length).toBeGreaterThan(0);
  expect(tokens[walletAddress]).toBeDefined();
  expect(tokens[walletAddress]).toBe(mintCount);

  const anotherAddress = WalletV1.calculateWalletAddress({
    pubKey: pubkey,
    shardId: 2,
    salt: 200n,
  });

  const transferCount = 100_000n;

  const sendHash = await wallet.sendMessage({
    to: anotherAddress,
    value: 10_000_000n,
    feeCredit: 100_000n * gasPriceOnShard2,
    tokens: [
      {
        id: walletAddress,
        amount: transferCount,
      },
    ],
  });

  await waitTillCompleted(client, 1, sendHash);

  const anotherTokens = await client.getCurrencies(bytesToHex(anotherAddress), "latest");

  expect(anotherTokens).toBeDefined();
  expect(Object.keys(anotherTokens).length).toBeGreaterThan(0);
  expect(anotherTokens[walletAddress]).toBeDefined();
  expect(anotherTokens[walletAddress]).toBe(transferCount);
});
