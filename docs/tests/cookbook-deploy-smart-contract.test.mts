import { RPC_GLOBAL } from "./globals";

//startImportStatements
import {
  ExternalMessageEnvelope,
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  bytesToHex,
  convertEthToWei,
  externalDeploymentMessage,
  generateRandomPrivateKey,
  hexToBytes,
  waitTillCompleted,
} from "@nilfoundation/niljs";

import { encodeFunctionData, type Abi } from "viem";
//endImportStatements

const RPC_ENDPOINT = RPC_GLOBAL;

import fs from "node:fs/promises";
import path from "node:path";
import { COUNTER_COMPILATION_COMMAND } from "./compilationCommands";
const __dirname = path.dirname(__filename);

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

let COUNTER_BYTECODE;

let COUNTER_ABI;

let COUNTER_ADDRESS: `0x${string}`;

beforeAll(async () => {
  await exec(COUNTER_COMPILATION_COMMAND);
  const counterFile = await fs.readFile(path.resolve(__dirname, "./Counter/Counter.bin"), "utf8");
  const counterBytecode = `0x${counterFile}` as `0x${string}`;
  const counterAbiFile = await fs.readFile(
    path.resolve(__dirname, "./Counter/Counter.abi"),
    "utf8",
  );
  const counterAbi = JSON.parse(counterAbiFile) as unknown as Abi;

  COUNTER_BYTECODE = counterBytecode;
  COUNTER_ABI = counterAbi;
});

describe.sequential("Nil.js passes the deployment and calling flow", async () => {
  test.sequential(
    "Nil.js can deploy Counter internally",
    async () => {
      //startInternalDeployment
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

      await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(10));

      await wallet.selfDeploy(true);

      const { address, hash } = await wallet.deployContract({
        bytecode: COUNTER_BYTECODE,
        abi: COUNTER_ABI as unknown as Abi,
        args: [],
        feeCredit: 10_000_000n,
        salt: SALT,
        shardId: 1,
      });

      const manufacturerReceipts = await waitTillCompleted(client, hash);
      //endInternalDeployment

      COUNTER_ADDRESS = address;

      expect(manufacturerReceipts.some((receipt) => !receipt.success)).toBe(false);

      const code = await client.getCode(address, "latest");

      expect(code).toBeDefined();
      expect(code.length).toBeGreaterThan(10);
    },
    40000,
  );

  test.sequential("Nil.js can deploy Counter externally", async () => {
    //startExternalDeployment
    const SALT = BigInt(Math.floor(Math.random() * 10000));

    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const faucet = new Faucet(client);

    const chainId = await client.chainId();

    const deploymentMessage = externalDeploymentMessage(
      {
        salt: SALT,
        shard: 1,
        bytecode: COUNTER_BYTECODE,
        abi: COUNTER_ABI as unknown as Abi,
        args: [],
      },
      chainId,
    );

    const addr = bytesToHex(deploymentMessage.to);

    const faucetHash = await faucet.withdrawToWithRetry(addr, convertEthToWei(0.1));

    await waitTillCompleted(client, faucetHash);

    const hash = await deploymentMessage.send(client);

    const receipts = await waitTillCompleted(client, hash);
    //endExternalDeployment

    expect(receipts.some((receipt) => !receipt.success)).toBe(false);

    const code = await client.getCode(addr, "latest");

    expect(code).toBeDefined();
    expect(code.length).toBeGreaterThan(10);
  });

  test.sequential("Nil.js can call Counter successfully with an internal message", async () => {
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

    await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(10));

    await wallet.selfDeploy(true);

    //startInternalMessage
    const hash = await wallet.sendMessage({
      to: COUNTER_ADDRESS,
      abi: COUNTER_ABI as unknown as Abi,
      feeCredit: 5_000_000n,
      functionName: "increment",
    });

    const receipts = await waitTillCompleted(client, hash);
    //endInternalMessage

    expect(receipts.some((receipt) => !receipt.success)).toBe(false);
  });

  test.sequential(
    "Nil.js can call Counter successfully with an external message",
    async () => {
      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const faucet = new Faucet(client);

      await faucet.withdrawToWithRetry(COUNTER_ADDRESS, convertEthToWei(10));

      const chainId = await client.chainId();
      //startExternalMessage
      const message = new ExternalMessageEnvelope({
        to: hexToBytes(COUNTER_ADDRESS),
        isDeploy: false,
        chainId,
        data: hexToBytes(
          encodeFunctionData({
            abi: COUNTER_ABI as unknown as Abi,
            functionName: "increment",
            args: [],
          }),
        ),
        authData: new Uint8Array(0),
        seqno: await client.getMessageCount(COUNTER_ADDRESS),
      });

      const encodedMessage = message.encode();

      let success = false;
      let messageHash: `0x${string}`;

      while (!success) {
        try {
          messageHash = await client.sendRawMessage(bytesToHex(encodedMessage));
          success = true;
        } catch (error) {
          await new Promise((resolve) => setTimeout(resolve, 1000));
        }
      }

      const receipts = await waitTillCompleted(client, messageHash);
      //endExternalMessage

      expect(receipts.some((receipt) => !receipt.success)).toBe(false);
    },
    40000,
  );
});
