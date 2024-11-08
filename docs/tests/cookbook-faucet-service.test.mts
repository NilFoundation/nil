import { RPC_GLOBAL, FAUCET_GLOBAL } from "./globals";

//startImportStatements
import {
  Faucet,
  FaucetClient,
  generateRandomPrivateKey,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
} from "@nilfoundation/niljs";

import {} from "viem";
//endImportStatements

let RPC_ENDPOINT = RPC_GLOBAL;

describe.sequential("Nil.js can use the faucet service", async () => {
  test.sequential("Nil.js can use the faucet service to do a default token top-up", async () => {
    //startDefaultExample
    const SALT = BigInt(Math.floor(Math.random() * 10000));

    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const faucet = new Faucet(client);

    const pkey = generateRandomPrivateKey();

    const signer = new LocalECDSAKeySigner({
      privateKey: pkey,
    });

    const pubkey = signer.getPublicKey();

    const wallet = new WalletV1({
      pubkey: pubkey,
      client: client,
      signer: signer,
      shardId: 1,
      salt: SALT,
    });

    const walletAddress = wallet.address;

    await faucet.withdrawToWithRetry(walletAddress, 80_000_000n);

    await wallet.selfDeploy(true);

    const resultBeforeTopUp = await client.getBalance(wallet.address);

    console.log(resultBeforeTopUp);

    //endDefaultExample

    RPC_ENDPOINT = FAUCET_GLOBAL;

    //startContDefaultExample

    const faucetClient = new FaucetClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
    });

    const faucets = await faucetClient.getAllFaucets();

    const defaultFaucet = faucets.NIL;

    const tx = await faucetClient.topUpAndWaitUntilCompletion(
      {
        faucetAddress: defaultFaucet,
        walletAddress,
        amount: 1_000_000,
      },
      client,
    );

    const result = await client.getBalance(wallet.address);

    console.log(result);
    console.log(tx);
    //endContDefaultExample
    expect(result > resultBeforeTopUp);
  });

  test.sequential(
    "Nil.js can use the faucet service to handle custom currencies top-up",
    async () => {
      RPC_ENDPOINT = RPC_GLOBAL;
      //startBTCExample
      const SALT = BigInt(Math.floor(Math.random() * 10000));

      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const faucet = new Faucet(client);

      const pkey = generateRandomPrivateKey();

      const signer = new LocalECDSAKeySigner({
        privateKey: pkey,
      });

      const pubkey = signer.getPublicKey();

      const wallet = new WalletV1({
        pubkey: pubkey,
        client: client,
        signer: signer,
        shardId: 1,
        salt: SALT,
      });

      const walletAddress = wallet.address;

      await faucet.withdrawToWithRetry(walletAddress, 80_000_000n);

      await wallet.selfDeploy(true);

      const resultBeforeTopUp = await client.getBalance(wallet.address);

      console.log(resultBeforeTopUp);

      //endBTCExample

      RPC_ENDPOINT = FAUCET_GLOBAL;

      //startContBTCExample
      const faucetClient = new FaucetClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
      });

      const faucets = await faucetClient.getAllFaucets();

      const faucetBTC = faucets.BTC;

      const tx = await faucetClient.topUpAndWaitUntilCompletion(
        {
          faucetAddress: faucetBTC,
          walletAddress,
          amount: 1_000_000,
        },
        client,
      );

      const result = await client.getCurrencies(walletAddress, "latest");

      console.log(result);

      //endContBTCExample

      expect(Object.values(result)).toContain(1_000_000n);
    },
    40000,
  );
});
