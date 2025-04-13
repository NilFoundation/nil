import { FAUCET_GLOBAL, RPC_GLOBAL } from "./globals";

//startImportStatements
import {
  ExternalTransactionEnvelope,
  HttpTransport,
  PublicClient,
  bytesToHex,
  externalDeploymentTransaction,
  generateSmartAccount,
  getContract,
  hexToBytes,
  topUp,
  waitTillCompleted,
} from "@nilfoundation/niljs";

import { type Abi, encodeFunctionData } from "viem";
//endImportStatements

const RPC_ENDPOINT = RPC_GLOBAL;
const FAUCET_ENDPOINT = FAUCET_GLOBAL;

import fs from "node:fs/promises";
import path from "node:path";
import util from "node:util";
import { expect } from "vitest";

import { COUNTER_COMPILATION_COMMAND } from "./compilationCommands";
const __dirname = path.dirname(__filename);
const exec = util.promisify(require("node:child_process").exec);

let COUNTER_BYTECODE: `0x${string}`;

let COUNTER_ABI: Abi;

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

      const gasPrice = await client.getGasPrice(1);

      const smartAccount = await generateSmartAccount({
        shardId: 1,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      const { address, tx } = await smartAccount.deployContract({
        bytecode: COUNTER_BYTECODE,
        abi: COUNTER_ABI,
        args: [],
        feeCredit: 1_000_000n * gasPrice,
        salt: SALT,
        shardId: 1,
      });

      const manufacturerReceipts = await tx.wait();
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

    const gasPrice = await client.getGasPrice(1);

    const chainId = await client.chainId();

    const deploymentTransaction = externalDeploymentTransaction(
      {
        salt: SALT,
        shard: 1,
        bytecode: COUNTER_BYTECODE,
        abi: COUNTER_ABI,
        args: [],
        feeCredit: 1000000n * gasPrice,
      },
      chainId,
    );

    const addr = bytesToHex(deploymentTransaction.to);

    await topUp({
      address: addr,
      faucetEndpoint: FAUCET_ENDPOINT,
      rpcEndpoint: RPC_ENDPOINT,
    });

    const hash = await deploymentTransaction.send(client);

    const receipts = await waitTillCompleted(client, hash);
    //endExternalDeployment

    expect(receipts.some((receipt) => !receipt.success)).toBe(false);

    const code = await client.getCode(addr, "latest");

    expect(code).toBeDefined();
    expect(code.length).toBeGreaterThan(10);
  });

  test.sequential(
    "contract factory can call Counter successfully",
    async () => {
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

      //startFactoryIncrement
      const contract = getContract({
        client: client,
        abi: COUNTER_ABI as unknown[],
        address: COUNTER_ADDRESS,
        smartAccount: smartAccount,
      });

      const res = await contract.read.getValue([]);
      expect(res).toBe(0n);

      const hash = await contract.write.increment([]);
      await waitTillCompleted(client, hash);

      const res2 = await contract.read.getValue([]);

      console.log(res2);
      //endFactoryIncrement

      expect(res).toBeDefined;
      expect(res2).toBeDefined;
      expect(res2).toBe(1n);
    },
    80000,
  );

  test.sequential("Nil.js can call Counter successfully with an internal transaction", async () => {
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

    //startInternalTransaction
    const tx = await smartAccount.sendTransaction({
      to: COUNTER_ADDRESS,
      abi: COUNTER_ABI,
      functionName: "increment",
    });

    const receipts = await tx.wait();
    //endInternalTransaction

    expect(receipts.some((receipt) => !receipt.success)).toBe(false);
  });

  test.sequential(
    "Nil.js can call Counter successfully with an external transaction",
    async () => {
      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      await topUp({
        address: COUNTER_ADDRESS,
        faucetEndpoint: FAUCET_ENDPOINT,
        rpcEndpoint: RPC_ENDPOINT,
      });

      const chainId = await client.chainId();
      //startExternalTransaction
      const transaction = new ExternalTransactionEnvelope({
        to: hexToBytes(COUNTER_ADDRESS),
        isDeploy: false,
        chainId,
        data: hexToBytes(
          encodeFunctionData({
            abi: COUNTER_ABI,
            functionName: "increment",
            args: [],
          }),
        ),
        authData: new Uint8Array(0),
        seqno: await client.getTransactionCount(COUNTER_ADDRESS),
      });

      const encodedTransaction = transaction.encode();

      let success = false;
      let transactionHash: `0x${string}`;

      while (!success) {
        try {
          transactionHash = await client.sendRawTransaction(bytesToHex(encodedTransaction));
          success = true;
        } catch (error) {
          await new Promise((resolve) => setTimeout(resolve, 1000));
        }
      }

      const receipts = await waitTillCompleted(client, transactionHash);
      //endExternalTransaction

      expect(receipts.some((receipt) => !receipt.success)).toBe(false);
    },
    40000,
  );
});
