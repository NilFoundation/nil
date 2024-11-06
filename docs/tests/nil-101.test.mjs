import { NIL_GLOBAL } from "./globals";
import {
  PRIVATE_KEY_PATTERN,
  RPC_PATTERN,
  NEW_WALLET_PATTERN,
  WALLET_BALANCE_PATTERN,
  CONTRACT_ADDRESS_PATTERN,
  ADDRESS_PATTERN,
  MESSAGE_HASH_PATTERN,
  WALLET_ADDRESS_PATTERN,
  CURRENCY_PATTERN,
  FAUCET_PATTERN,
} from "./patterns";
import { COUNTER_COMPILATION_COMMAND, CALLER_COMPILATION_COMMAND } from "./compilationCommands";
import TestHelper from "./TestHelper";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

let SALT = BigInt(Math.floor(Math.random() * 10000));

const CONFIG_FILE_NAME = "./tests/tempConfigNil101.ini";

const CONFIG_FLAG = `--config ${CONFIG_FILE_NAME}`;

let TEST_COMMANDS;
let COUNTER_ADDRESS;
let CALLER_ADDRESS;
let NEW_WALLET_ADDRESS;

beforeAll(async () => {
  const testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  TEST_COMMANDS = testHelper.createCLICommandsMap(SALT);
  await exec(TEST_COMMANDS["CONFIG_COMMAND"]);
});

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.sequential("initial wallet setup tests", () => {
  test.sequential("keygen generation works via CLI", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["KEYGEN_COMMAND"]);
    expect(stdout).toMatch(PRIVATE_KEY_PATTERN);
  });

  test.sequential("endpoint command should set the endpoint", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["RPC_COMMAND"]);
    expect(stderr).toMatch(RPC_PATTERN);
  });

  test.sequential("faucet_endpoint command should set the faucet endpoint", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["FAUCET_COMMAND"]);
    expect(stderr).toMatch(FAUCET_PATTERN);
  });

  test.sequential("wallet creation command creates a wallet", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["WALLET_CREATION_COMMAND"]);
    expect(stdout).toMatch(NEW_WALLET_PATTERN);
  });

  test.sequential("wallet top-up command tops up the wallet", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["WALLET_TOP_UP_COMMAND"]);
    expect(stdout).toMatch(WALLET_BALANCE_PATTERN);
  });
});

describe.sequential("incrementer tests", () => {
  test.sequential("wallet info command supplies info", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["WALLET_INFO_COMMAND"]);
    expect(stdout).toMatch(WALLET_ADDRESS_PATTERN);
  });

  test.sequential("deploy of incrementer works successfully", async () => {
    await exec(COUNTER_COMPILATION_COMMAND);
    const { stdout, stderr } = await exec(TEST_COMMANDS["COUNTER_DEPLOYMENT_COMMAND"]);
    expect(stdout).toMatch(CONTRACT_ADDRESS_PATTERN);
    const addressMatches = stdout.match(ADDRESS_PATTERN);
    COUNTER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;
  });

  test.sequential("execution of increment produces a message", async () => {
    //startIncrement
    const COUNTER_INCREMENT_COMMAND = `${NIL_GLOBAL} wallet send-message ${COUNTER_ADDRESS} increment --abi ./tests/Counter/Counter.abi ${CONFIG_FLAG}`;
    //endIncrement
    const { stdout, stderr } = await exec(COUNTER_INCREMENT_COMMAND);
    expect(stdout).toMatch(MESSAGE_HASH_PATTERN);
  });

  test.sequential("call to incrementer returns the correct value", async () => {
    //start_CallToIncrementer
    const COUNTER_CALL_READONLY_COMMAND = `${NIL_GLOBAL} contract call-readonly ${COUNTER_ADDRESS} getValue --abi ./tests/Counter/Counter.abi ${CONFIG_FLAG}`;
    //end_CallToIncrementer
    const { stdout, stderr } = await exec(COUNTER_CALL_READONLY_COMMAND);

    const normalize = (str) => str.replace(/\r\n/g, "\n").trim();

    const expectedOutput = "Success, result:\nuint256: 1";
    const receivedOutput = normalize(stdout);

    expect(receivedOutput).toBe(expectedOutput);
  });
});

describe.sequential("caller tests", () => {
  beforeEach(() => {
    SALT = BigInt(Math.floor(Math.random() * 10000));
  });
  test.sequential("deploy of caller works successfully", async () => {
    await exec(CALLER_COMPILATION_COMMAND);
    const { stdout, stderr } = await exec(TEST_COMMANDS["CALLER_DEPLOYMENT_COMMAND"]);
    const addressMatches = stdout.match(ADDRESS_PATTERN);
    CALLER_ADDRESS = addressMatches && addressMatches.length > 0 ? addressMatches[1] : null;
    expect(CALLER_ADDRESS).not.toBeNull();
  });

  test.sequential("caller can call incrementer successfully", async () => {
    //start_SendTokensToCaller
    const SEND_TOKENS_COMMAND = `${NIL_GLOBAL} wallet send-tokens ${CALLER_ADDRESS} 3000000 ${CONFIG_FLAG}`;
    //end_SendTokensToCaller

    //startMessageFromCallerToIncrementer
    const SEND_FROM_CALLER_COMMAND = `${NIL_GLOBAL} wallet send-message ${CALLER_ADDRESS} call ${COUNTER_ADDRESS} --abi ./tests/Caller/Caller.abi ${CONFIG_FLAG}`;
    //endMessageFromCallerToIncrementer

    await exec(SEND_TOKENS_COMMAND);
    const { stdout, stderr } = await exec(SEND_FROM_CALLER_COMMAND);
    expect(stdout).toMatch(MESSAGE_HASH_PATTERN);

    const COUNTER_CALL_READONLY_COMMAND_POST_CALLER = `${NIL_GLOBAL} contract call-readonly ${COUNTER_ADDRESS} getValue --abi ./tests/Counter/Counter.abi ${CONFIG_FLAG}`;

    let stdoutCall;
    let stderrCall;

    try {
      for (let attempt = 0; attempt < 5; attempt++) {
        ({ stdout: stdoutCall, stderr: stderrCall } = await exec(
          COUNTER_CALL_READONLY_COMMAND_POST_CALLER,
        ));

        if (stdoutCall) {
          break;
        }

        console.log(`Attempt ${attempt + 1}: Retrying after a short delay...`);
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }

      if (!stdoutCall) {
        throw new Error("Failed to get output from the contract call after multiple attempts.");
      }

      const normalize = (str) => str.replace(/\r\n/g, "\n").trim();

      const expectedOutput = "Success, result:\nuint256: 2";
      const receivedOutput = normalize(stdoutCall);

      expect(receivedOutput).toBe(expectedOutput);
    } catch (error) {
      console.error("Error during the contract call:", error);
      if (stderrCall) {
        console.error("stderrCall:", stderrCall);
      }
      throw error;
    }
  });
});

describe.sequential("tokens tests", () => {
  test.sequential("a new wallet is created successfully", async () => {
    const { stdout, stderr } = await exec(TEST_COMMANDS["WALLET_CREATION_COMMAND_WITH_SALT"]);
    expect(stdout).toMatch(WALLET_ADDRESS_PATTERN);
    const addressMatches = stdout.match(WALLET_ADDRESS_PATTERN);
    NEW_WALLET_ADDRESS = addressMatches && addressMatches.length > 0 ? addressMatches[0] : null;
  });

  test.sequential("a new currency is created and withdrawn successfully", async () => {
    //startMintCurrency
    const MINT_CURRENCY_COMMAND = `${NIL_GLOBAL} minter create-currency ${NEW_WALLET_ADDRESS} 5000 new-currency ${CONFIG_FLAG}`;
    //endMintCurrency

    await exec(MINT_CURRENCY_COMMAND);

    //startCurrenciesCheck
    const CURRENCIES_COMMAND = `${NIL_GLOBAL} contract currencies ${NEW_WALLET_ADDRESS} ${CONFIG_FLAG}`;
    //endCurrenciesCheck

    const { stdout, stderr } = await exec(CURRENCIES_COMMAND);
    expect(stdout).toMatch(CURRENCY_PATTERN);
  });
});
