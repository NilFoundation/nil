import { COUNTER_BUG_COMPILATION_COMMAND } from "./compilationCommands";
import {
  HASH_PATTERN,
  ADDRESS_PATTERN,
  SUCCESSFUL_EXECUTION_PATTERN,
  COUNTER_BUG_DEBUG_PATTERN,
} from "./patterns";
import { COMETA_GLOBAL, NIL_GLOBAL, RPC_GLOBAL } from "./globals";
import TestHelper from "./TestHelper";
//startNilJSImport
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const {
  CometaService,
  convertEthToWei,
  Faucet,
  generateRandomPrivateKey,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  waitTillCompleted,
  WalletV1,
} = require("@nilfoundation/niljs");
//endNilJSImport
import type { Abi } from "viem";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

const RPC_ENDPOINT = RPC_GLOBAL;
const COMETA_ENDPOINT = COMETA_GLOBAL;

const SALT = BigInt(Math.floor(Math.random() * 10000));

const CONFIG_FILE_NAME = "./tests/tempConfigCometaDebug.ini";

const CONFIG_FLAG = `--config ${CONFIG_FILE_NAME}`;

let TEST_COMMANDS: {};
let COUNTER_BUG_ADDRESS: string;
let COUNTER_BUG_ADDRESS_SEPARATE: string;
let MESSAGE_HASH;

beforeAll(async () => {
  const testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  TEST_COMMANDS = testHelper.createCLICommandsMap(SALT);
  await testHelper.prepareTestCLI();
});

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.skip.sequential("CLI tutorial flows pass correctly for CounterBug", () => {
  test.sequential("CLI can compile, deploy, and register CounterBug in one command", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["COUNTER_BUG_COMETA_COMMAND"]);
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(ADDRESS_PATTERN);
    const addressMatches = stdout.match(ADDRESS_PATTERN);
    COUNTER_BUG_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;
  });

  test.sequential(
    "CLI can compile, deploy, and register CounterBug in multiple commands",
    async () => {
      let { stdout, stderr } = await exec(COUNTER_BUG_COMPILATION_COMMAND);
      expect(stdout).toBeDefined;
      expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
      ({ stdout, stderr } = await exec(TEST_COMMANDS["COUNTER_BUG_DEPLOYMENT_COMMAND"]));
      expect(stdout).toBeDefined;
      expect(stdout).toMatch(ADDRESS_PATTERN);
      const addressMatches = stdout.match(ADDRESS_PATTERN);
      COUNTER_BUG_ADDRESS_SEPARATE = addressMatches.length > 1 ? addressMatches[1] : null;
      //startCounterBugRegistrationCommand
      const COUNTER_BUG_REGISTRATION_COMMAND = `${NIL_GLOBAL} cometa register --address ${COUNTER_BUG_ADDRESS_SEPARATE} --compile-input ./tests/counter.json ${CONFIG_FLAG}`;
      //endCounterBugRegistrationCommand
      ({ stdout, stderr } = await exec(COUNTER_BUG_REGISTRATION_COMMAND));
    },
  );

  test.sequential("CLI calls CounterBug and produces a message", async () => {
    //startCounterBugIncrementCommand
    const COUNTER_BUG_INCREMENT_COMMAND = `${NIL_GLOBAL} wallet send-message ${COUNTER_BUG_ADDRESS} increment --abi ./tests/CounterBug/CounterBug.abi ${CONFIG_FLAG}`;
    //endCounterBugIncrementCommand
    const { stdout, stderr } = await exec(COUNTER_BUG_INCREMENT_COMMAND);
    expect(stdout.toBeDefined);
    expect(stdout).toMatch(HASH_PATTERN);
    MESSAGE_HASH = stdout.match(HASH_PATTERN)[0];
  });

  test.skip.sequential("debugging the message shows where the contract failed", async () => {
    //startDebugCommand
    const DEBUG_COMMAND = `${NIL_GLOBAL} debug ${MESSAGE_HASH}`;
    //endDebugCommand

    const { stdout, stderr } = await exec(DEBUG_COMMAND);
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(COUNTER_BUG_DEBUG_PATTERN);
  });
});

describe.skip.sequential("Nil.js correctly interacts with Cometa", () => {
  test.sequential("Nil.js passes the Cometa tutorial flow", async () => {
    //startNilJSCometaTutorialSnippet
    const cometa = new CometaService({
      transport: new HttpTransport({
        endpoint: COMETA_ENDPOINT,
      }),
    });

    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const faucet = new Faucet(client);
    const signer = new LocalECDSAKeySigner({
      privateKey: generateRandomPrivateKey(),
    });

    const pubkey = await signer.getPublicKey();
    const wallet = new WalletV1({
      pubkey: pubkey,
      salt: BigInt(Math.floor(Math.random() * 10000)),
      shardId: 1,
      client,
      signer,
    });

    const walletAddress = wallet.address;
    await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(1));
    await wallet.selfDeploy(true);

    const counterBugJson = `{
        "contractName": "CounterBug.sol:CounterBug",
        "compilerVersion": "0.8.21",
        "settings": {
          "evmVersion": "shanghai",
          "optimizer": {
            "enabled": false,
            "runs": 200
          }
        },
        "sources": {
          "CounterBug.sol": {
            "urls": ["./CounterBug.sol"]
          }

        }
      }`;

    const compilationResult = await cometa.compileContract(counterBugJson);

    const { address, hash } = await wallet.deployContract({
      bytecode: compilationResult.code,
      abi: compilationResult.abi as unknown as Abi,
      args: [],
      salt: BigInt(Math.floor(Math.random() * 10000)),
      feeCredit: 500_000n,
      shardId: 1,
    });

    const receipts = await waitTillCompleted(client, 1, hash);

    if (receipts.some((receipt) => !receipt.success)) {
      throw new Error("Contract deployment failed");
    }

    await cometa.registerContract(compilationResult, address);

    const incrementHash = await wallet.sendMessage({
      to: address,
      functionName: "increment",
      abi: compilationResult.abi as unknown as Abi,
      feeCredit: 300_000n,
    });

    await waitTillCompleted(client, 1, incrementHash);

    //endNilJSCometaTutorialSnippet
  });
});
