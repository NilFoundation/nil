import fs from "node:fs/promises";
import path from "node:path";
import { CLONE_FACTORY_COMPILATION_COMMAND } from "./compilationCommands";
import type { Abi } from "viem";
import { RPC_GLOBAL } from "./globals";
import {
  convertEthToWei,
  Faucet,
  generateRandomPrivateKey,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  waitTillCompleted,
  WalletV1,
} from "@nilfoundation/niljs";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

const __dirname = path.dirname(__filename);

const RPC_ENDPOINT = RPC_GLOBAL;

let FACTORY_MANAGER_BYTECODE;
let FACTORY_MANAGER_ABI;

let MASTER_CHILD_BYTECODE;
let MASTER_CHILD_ABI;

let CLONE_FACTORY_BYTECODE;
let CLONE_FACTORY_ABI;

beforeAll(async () => {
  await exec(CLONE_FACTORY_COMPILATION_COMMAND);
  const masterChildFile = await fs.readFile(
    path.resolve(__dirname, "./CloneFactory/MasterChild.bin"),
    "utf8",
  );
  const masterChildBytecode = `0x${masterChildFile}` as `0x${string}`;
  const masterChildAbiFile = await fs.readFile(
    path.resolve(__dirname, "./CloneFactory/MasterChild.abi"),
    "utf8",
  );
  const masterChildAbi = JSON.parse(masterChildAbiFile) as unknown as Abi;

  const cloneFactoryFile = await fs.readFile(
    path.resolve(__dirname, "./CloneFactory/CloneFactory.bin"),
    "utf8",
  );
  const cloneFactoryBytecode = `0x${cloneFactoryFile}` as `0x${string}`;
  const cloneFactoryAbiFile = await fs.readFile(
    path.resolve(__dirname, "./CloneFactory/CloneFactory.abi"),
    "utf8",
  );
  const cloneFactoryAbi = JSON.parse(cloneFactoryAbiFile) as unknown as Abi;

  const factoryManagerFile = await fs.readFile(
    path.resolve(__dirname, "./CloneFactory/FactoryManager.bin"),
    "utf8",
  );
  const factoryManagerBytecode = `0x${factoryManagerFile}` as `0x${string}`;
  const factoryManagerAbiFile = await fs.readFile(
    path.resolve(__dirname, "./CloneFactory/FactoryManager.abi"),
    "utf8",
  );
  const factoryManagerAbi = JSON.parse(factoryManagerAbiFile) as unknown as Abi;

  FACTORY_MANAGER_BYTECODE = factoryManagerBytecode;
  FACTORY_MANAGER_ABI = factoryManagerAbi;
  MASTER_CHILD_BYTECODE = masterChildBytecode;
  MASTER_CHILD_ABI = masterChildAbi;
  CLONE_FACTORY_BYTECODE = cloneFactoryBytecode;
  CLONE_FACTORY_ABI = cloneFactoryAbi;
});

describe.sequential("Nil.js can fully tests the CloneFactory", async () => {
  test("CloneFactory successfully creates a factory and a clone", async () => {
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

    const { address: factoryManagerAddress, hash: factoryManagerHash } =
      await wallet.deployContract({
        bytecode: FACTORY_MANAGER_BYTECODE,
        abi: FACTORY_MANAGER_ABI,
        args: [],
        feeCredit: 50_000_000n,
        salt: SALT,
        shardId: 1,
      });

    const factoryManagerReceipts = await waitTillCompleted(client, factoryManagerHash);

    expect(factoryManagerReceipts.some((receipt) => !receipt.success)).toBe(false);

    const createMasterChildHash = await wallet.sendMessage({
      to: factoryManagerAddress,
      feeCredit: 10_000_000n,
      abi: FACTORY_MANAGER_ABI,
      functionName: "deployNewMasterChild",
      args: [2, SALT],
    });

    const createMasterChildReceipts = await waitTillCompleted(client, createMasterChildHash);

    const masterChildAddress = createMasterChildReceipts[2].contractAddress as `0x${string}`;

    console.log(masterChildAddress);

    expect(createMasterChildReceipts.some((receipt) => !receipt.success)).toBe(false);

    const createFactoryHash = await wallet.sendMessage({
      to: factoryManagerAddress,
      feeCredit: 10_000_000n,
      abi: FACTORY_MANAGER_ABI,
      functionName: "deployNewFactory",
      args: [2, SALT],
    });

    const createFactoryReceipts = await waitTillCompleted(client, createFactoryHash);

    const factoryAddress = createFactoryReceipts[2].contractAddress as `0x${string}`;

    console.log(factoryAddress);

    expect(createFactoryReceipts.some((receipt) => !receipt.success)).toBe(false);

    const createCloneHash = await wallet.sendMessage({
      to: factoryAddress,
      feeCredit: 5_000_000n,
      abi: CLONE_FACTORY_ABI,
      functionName: "createCounterClone",
      args: [SALT],
    });

    const createCloneReceipts = await waitTillCompleted(client, createCloneHash);

    const cloneAddress = createCloneReceipts[2].contractAddress as `0x${string}`;

    expect(createCloneReceipts.some((receipt) => !receipt.success)).toBe(false);

    const incrementHash = await wallet.sendMessage({
      to: cloneAddress as `0x${string}`,
      abi: MASTER_CHILD_ABI,
      functionName: "increment",
      args: [],
      feeCredit: 3_000_000n,
    });

    console.log(cloneAddress);

    const incrementReceipts = await waitTillCompleted(client, incrementHash);

    expect(incrementReceipts.some((receipt) => !receipt.success)).toBe(false);

    const result = await client.call(
      {
        to: cloneAddress,
        functionName: "getValue",
        abi: MASTER_CHILD_ABI,
        feeCredit: 1_000_000n,
      },
      "latest",
    );

    console.log(result);

    expect(result.decodedData).toBe(1n);
  }, 80000);
});
