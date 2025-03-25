import TestHelper from "./TestHelper";
import { COUNTER_BUG_COMPILATION_COMMAND } from "./compilationCommands";
import { COMETA_GLOBAL, FAUCET_GLOBAL, NIL_GLOBAL, RPC_GLOBAL } from "./globals";
import {
  ADDRESS_PATTERN,
  COUNTER_BUG_DEBUG_PATTERN,
  HASH_PATTERN,
  SUCCESSFUL_EXECUTION_PATTERN,
} from "./patterns";

//startNilJSImport
import {
  CometaClient,
  HttpTransport,
  PublicClient,
  generateSmartAccount,
} from "@nilfoundation/niljs";
//endNilJSImport

import util from "node:util";
import type { Abi } from "viem";
import type COMMANDS from "./commands.mjs";

const exec = util.promisify(require("node:child_process").exec);

const RPC_ENDPOINT = RPC_GLOBAL;
const FAUCET_ENDPOINT = FAUCET_GLOBAL;
const COMETA_ENDPOINT = COMETA_GLOBAL;

const SALT = BigInt(Math.floor(Math.random() * 10000));

const CONFIG_FILE_NAME = "./tests/tempConfigCometaDebug.ini";

const CONFIG_FLAG = `--config ${CONFIG_FILE_NAME}`;

let TEST_COMMANDS: typeof COMMANDS;
let COUNTER_BUG_ADDRESS: string;
let COUNTER_BUG_ADDRESS_SEPARATE: string;
let TRANSACTION_HASH: string;

beforeAll(async () => {
  const testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  TEST_COMMANDS = testHelper.createCLICommandsMap(SALT);
  await testHelper.prepareTestCLI();
});

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.sequential("CLI tutorial flows pass correctly for CounterBug", () => {
  test.sequential("CLI can compile, deploy, and register CounterBug in one command", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS.COUNTER_BUG_COMETA_COMMAND);
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
      ({ stdout, stderr } = await exec(TEST_COMMANDS.COUNTER_BUG_DEPLOYMENT_COMMAND));
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

  test.sequential("CLI calls CounterBug and produces a transaction", async () => {
    //startCounterBugIncrementCommand
    const COUNTER_BUG_INCREMENT_COMMAND = `${NIL_GLOBAL} smart-account send-transaction ${COUNTER_BUG_ADDRESS} increment --abi ./tests/CounterBug/CounterBug.abi ${CONFIG_FLAG}`;
    //endCounterBugIncrementCommand
    const { stdout, stderr } = await exec(COUNTER_BUG_INCREMENT_COMMAND);
    expect(stdout.toBeDefined);
    expect(stdout).toMatch(HASH_PATTERN);
    TRANSACTION_HASH = stdout.match(HASH_PATTERN)[0];
  });

  test.sequential("debugging the transaction shows where the contract failed", async () => {
    //startDebugCommand
    const DEBUG_COMMAND = `${NIL_GLOBAL} debug ${TRANSACTION_HASH} ${CONFIG_FLAG}`;
    //endDebugCommand

    const { stdout, stderr } = await exec(DEBUG_COMMAND);
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(COUNTER_BUG_DEBUG_PATTERN);
  });
});

describe.skip.sequential("Nil.js correctly interacts with Cometa", () => {
  test.sequential("Nil.js passes the Cometa tutorial flow", async () => {
    //startNilJSCometaTutorialSnippet
    const cometa = new CometaClient({
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

    const smartAccount = await generateSmartAccount({
      shardId: 1,
      rpcEndpoint: RPC_ENDPOINT,
      faucetEndpoint: FAUCET_ENDPOINT,
    });

    const counterBugJson = `{
        "contractName": "CounterBug.sol:CounterBug",
        "compilerVersion": "0.8.28",
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

    const { address, tx } = await smartAccount.deployContract({
      bytecode: compilationResult.code,
      abi: compilationResult.abi as unknown as Abi,
      args: [],
      salt: BigInt(Math.floor(Math.random() * 10000)),
      feeCredit: 500_000n,
      shardId: 1,
    });

    const receipts = await tx.wait();

    if (receipts.some((receipt) => !receipt.success)) {
      throw new Error("Contract deployment failed");
    }

    await cometa.registerContractData(compilationResult, address);

    const incrementTx = await smartAccount.sendTransaction({
      to: address,
      functionName: "increment",
      abi: compilationResult.abi as unknown as Abi,
      feeCredit: 300_000n,
    });

    await incrementTx.wait();

    //endNilJSCometaTutorialSnippet
  });
});
