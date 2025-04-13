import {
  ExternalTransactionEnvelope,
  HttpTransport,
  PublicClient,
  bytesToHex,
  convertEthToWei,
  externalDeploymentTransaction,
  generateRandomPrivateKey,
  generateSmartAccount,
  getPublicKey,
  hexToBytes,
  topUp,
  waitTillCompleted,
} from "@nilfoundation/niljs";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

import { FAUCET_GLOBAL, RPC_GLOBAL } from "./globals";

import TestHelper from "./TestHelper";

import {
  MANUFACTURER_COMPILATION_COMMAND,
  RETAILER_COMPILATION_COMMAND,
} from "./compilationCommands";
import { MANUFACTURER_COMPILATION_PATTERN, RETAILER_COMPILATION_PATTERN } from "./patterns";

const RPC_ENDPOINT = RPC_GLOBAL;
const FAUCET_ENDPOINT = FAUCET_GLOBAL;

import { decodeFunctionResult, encodeFunctionData } from "viem";

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const CONFIG_FILE_NAME = "./tests/tempWorkingWithSmartContractsNilJS.ini";

let RETAILER_BYTECODE;
let RETAILER_ABI;
let MANUFACTURER_BYTECODE;
let MANUFACTURER_ABI;

beforeAll(async () => {
  const testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  await testHelper.prepareTestCLI();
});

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.sequential("Nil.js deployment tests", async () => {
  test.sequential("compiling of Retailer and Manufacturer is successful", async () => {
    const __filename = fileURLToPath(import.meta.url);
    const __dirname = path.dirname(__filename);

    let { stdout, stderr } = await exec(RETAILER_COMPILATION_COMMAND);
    expect(stderr).toMatch(RETAILER_COMPILATION_PATTERN);
    ({ stdout, stderr } = await exec(MANUFACTURER_COMPILATION_COMMAND));
    expect(stdout).toMatch(MANUFACTURER_COMPILATION_PATTERN);

    RETAILER_BYTECODE = `0x${fs.readFileSync(path.join(__dirname, "./Retailer/Retailer.bin"), "utf-8")}`;
    RETAILER_ABI = JSON.parse(
      fs.readFileSync(path.join(__dirname, "./Retailer/Retailer.abi"), "utf-8"),
    );
    MANUFACTURER_BYTECODE = `0x${fs.readFileSync(path.join(__dirname, "./Manufacturer/Manufacturer.bin"), "utf-8")}`;
    MANUFACTURER_ABI = JSON.parse(
      fs.readFileSync(path.join(__dirname, "./Manufacturer/Manufacturer.abi"), "utf-8"),
    );
  });

  test.skip.sequential(
    "the contract factory works correctly with Retailer and Manufacturer",
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

      const { address: retailerAddress, tx: retailerDeploymenttx } =
        await smartAccount.deployContract({
          bytecode: RETAILER_BYTECODE,
          abi: RETAILER_ABI,
          args: [],
          value: 0n,
          feeCredit: 1_000_000n * gasPrice,
          salt: BigInt(Math.floor(Math.random() * 10000)),
          shardId: 1,
        });

      const receiptsRetailer = await retailerDeploymenttx.wait();
      expect(receiptsRetailer.some((receipt) => !receipt.success)).toBe(false);
      const retailerCode = await client.getCode(retailerAddress, "latest");

      expect(retailerCode).toBeDefined();
      expect(retailerCode.length).toBeGreaterThan(10);

      const { address: manufacturerAddress, tx: manufacturerDeploymenttx } =
        await smartAccount.deployContract({
          bytecode: MANUFACTURER_BYTECODE,
          abi: MANUFACTURER_ABI,
          args: [bytesToHex(pubkey), retailerAddress],
          value: 0n,
          feeCredit: 1000000n * gasPrice,
          salt: BigInt(Math.floor(Math.random() * 10000)),
          shardId: 2,
        });

      const manufacturerReceipts = await manufacturerDeploymenttx.wait();

      expect(manufacturerReceipts.some((receipt) => !receipt.success)).toBe(false);
      const manufacturerCode = await client.getCode(manufacturerAddress, "latest");

      expect(manufacturerCode).toBeDefined();
      expect(manufacturerCode.length).toBeGreaterThan(10);

      //startFactoryOrder

      const hashFunds = await faucet.withdrawToWithRetry(retailerAddress, convertEthToWei(1));

      await waitTillCompleted(client, hashFunds);

      const retailerContract = getContract({
        client,
        RETAILER_ABI,
        address: retailerAddress,
        smartAccount: smartAccount,
      });

      const manufacturerContract = getContract({
        client,
        MANUFACTURER_ABI,
        address: manufacturerAddress,
        smartAccount: smartAccount,
      });

      const res = await retailerContract.read.orderProduct([manufacturerAddress, "new-product"]);

      //endFactoryOrder

      //startFactoryProducts

      const res2 = await manufacturerContract.read.getProducts();

      console.log(res2);

      //endFactoryProducts

      expect(res).toBeDefined;
      expect(res2).toBeDefined;
      expect(res2).toContain(/new-product/);
    },
    80000,
  );

  test.skip.sequential(
    "internal deployment: Retailer and Manufacturer can exchange transactions",
    async () => {
      //startInternalDeployOfRetailer
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

      const { address: retailerAddress, tx: retailerDeploymenttx } =
        await smartAccount.deployContract({
          bytecode: RETAILER_BYTECODE,
          abi: RETAILER_ABI,
          args: [],
          value: 0n,
          feeCredit: 1_000_000n * gasPrice,
          salt: BigInt(Math.floor(Math.random() * 10000)),
          shardId: 1,
        });

      const receiptsRetailer = await retailerDeploymenttx.wait();
      //endInternalDeployOfRetailer
      expect(receiptsRetailer.some((receipt) => !receipt.success)).toBe(false);
      const retailerCode = await client.getCode(retailerAddress, "latest");

      expect(retailerCode).toBeDefined();
      expect(retailerCode.length).toBeGreaterThan(10);

      //startInternalDeployOfManufacturer

      const { address: manufacturerAddress, tx: manufacturerDeploymenttx } =
        await smartAccount.deployContract({
          bytecode: MANUFACTURER_BYTECODE,
          abi: MANUFACTURER_ABI,
          args: [bytesToHex(pubkey), retailerAddress],
          value: 0n,
          feeCredit: 1_000_000n * gasPrice,
          salt: BigInt(Math.floor(Math.random() * 10000)),
          shardId: 2,
        });

      const manufacturerReceipts = await manufacturerDeploymenttx.wait();
      //endInternalDeployOfManufacturer

      expect(manufacturerReceipts.some((receipt) => !receipt.success)).toBe(false);
      const manufacturerCode = await client.getCode(manufacturerAddress, "latest");

      expect(manufacturerCode).toBeDefined();
      expect(manufacturerCode.length).toBeGreaterThan(10);

      //startRetailerSendsTransactionToManufacturer
      const hashFunds = await faucet.withdrawToWithRetry(retailerAddress, convertEthToWei(1));

      await waitTillCompleted(client, hashFunds);

      const tx = await smartAccount.sendTransaction({
        to: retailerAddress,
        data: encodeFunctionData({
          abi: RETAILER_ABI,
          functionName: "orderProduct",
          args: [manufacturerAddress, "another-product"],
        }),
        feeCredit: 3_000_000n * gasPrice,
      });

      const productReceipts = await tx.wait();
      //endRetailerSendsTransactionToManufacturer

      expect(productReceipts.some((receipt) => !receipt.success)).toBe(false);

      //startRetailerRetrievesTheResult
      const resultsCall = await client.call(
        {
          from: manufacturerAddress,
          to: manufacturerAddress,
          data: encodeFunctionData({
            abi: MANUFACTURER_ABI,
            functionName: "getProducts",
            args: [],
          }),
        },
        "latest",
      );

      console.log(
        "getProducts",
        decodeFunctionResult({
          abi: MANUFACTURER_ABI,
          functionName: "getProducts",
          data: resultsCall.data,
        }),
      );
      //endRetailerRetrievesTheResult
    },
    50000,
  );

  test.sequential(
    "external deployment: Retailer and Manufacturer can exchange transactions",
    async () => {
      //startExternalDeployOfRetailer
      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const gasPrice = await client.getGasPrice(1);

      const pubkey = getPublicKey(generateRandomPrivateKey());

      const chainId = await client.chainId();

      const deploymentTransactionRetailer = externalDeploymentTransaction(
        {
          salt: BigInt(Math.floor(Math.random() * 10000)),
          shard: 1,
          bytecode: RETAILER_BYTECODE,
          feeCredit: 1_000_000n * gasPrice,
        },
        chainId,
      );

      const addressRetailer = bytesToHex(deploymentTransactionRetailer.to);

      await topUp({
        address: addressRetailer,
        faucetEndpoint: FAUCET_ENDPOINT,
        rpcEndpoint: RPC_ENDPOINT,
      });

      const receipts = await deploymentTransactionRetailer.send(client);

      await waitTillCompleted(client, receipts);
      //endExternalDeployOfRetailer

      const code = await client.getCode(addressRetailer, "latest");

      expect(code).toBeDefined();
      expect(code.length).toBeGreaterThan(10);

      //startExternalDeployOfManufacturer
      const clientTwo = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 2,
      });

      const gasPrice2 = await client.getGasPrice(2);

      const deploymentTransactionManufacturer = externalDeploymentTransaction(
        {
          salt: BigInt(Math.floor(Math.random() * 10000)),
          shard: 2,
          bytecode: MANUFACTURER_BYTECODE,
          abi: MANUFACTURER_ABI,
          args: [bytesToHex(pubkey), addressRetailer],
          feeCredit: 1_000_000n * gasPrice2,
        },
        chainId,
      );

      const addressManufacturer = bytesToHex(deploymentTransactionManufacturer.to);

      await topUp({
        address: addressManufacturer,
        faucetEndpoint: FAUCET_ENDPOINT,
        rpcEndpoint: RPC_ENDPOINT,
      });

      const receiptsManufacturer = await deploymentTransactionManufacturer.send(client);

      await waitTillCompleted(clientTwo, receiptsManufacturer);
      //endExternalDeployOfManufacturer

      const codeManufacturer = await client.getCode(addressManufacturer, "latest");

      expect(codeManufacturer).toBeDefined();
      expect(codeManufacturer.length).toBeGreaterThan(10);

      //startExternalSendTransactionToRetailer;
      const orderTransaction = new ExternalTransactionEnvelope({
        isDeploy: false,
        to: hexToBytes(addressRetailer),
        chainId,
        data: hexToBytes(
          encodeFunctionData({
            abi: RETAILER_ABI,
            functionName: "orderProduct",
            args: [addressManufacturer, "new-product"],
          }),
        ),
        authData: new Uint8Array(0),
        seqno: await client.getTransactionCount(addressRetailer),
      });

      const encodedOrderTransaction = orderTransaction.encode();

      let success = false;
      let ordertransactionHash;

      while (!success) {
        try {
          ordertransactionHash = await client.sendRawTransaction(
            bytesToHex(encodedOrderTransaction),
          );
          success = true;
        } catch (error) {
          await new Promise((resolve) => setTimeout(resolve, 1000));
        }
      }

      const orderReceipts = await waitTillCompleted(client, ordertransactionHash);
      //endExternalSendTransactionToRetailer
      expect(orderReceipts.some((receipt) => !receipt.success)).toBe(false);
    },
    50000,
  );
});
