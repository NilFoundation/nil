import { NIL_GLOBAL } from "./globals";
import {
  COUNTER_COMPILATION_COMMAND,
  CALLER_ASYNC_COMPILATION_COMMAND,
  CALLER_ASYNC_BP_COMPILATION_COMMAND,
  VALIDATOR_COMPILATION_COMMAND,
  ESCROW_COMPILATION_COMMAND,
  AWAITER_COMPILATION_COMMAND,
} from "./compilationCommands";
import {
  SUCCESSFUL_EXECUTION_PATTERN,
  ESCROW_SUCCESSFUL_PATTERN,
  RETAILER_COMPILATION_PATTERN,
  CONTRACT_ADDRESS_PATTERN,
  ADDRESS_PATTERN,
} from "./patterns";
import TestHelper from "./TestHelper";

const SALT = BigInt(Math.floor(Math.random() * 10000));

const CONFIG_FILE_NAME = "./tests/tempConfigAsyncTests.ini";

const CONFIG_FLAG = `--config ${CONFIG_FILE_NAME}`;

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

let TEST_COMMANDS;
let COUNTER_ADDRESS;
let AWAITER_ADDRESS;

beforeAll(async () => {
  const testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  TEST_COMMANDS = testHelper.createCLICommandsMap(SALT);
  await testHelper.prepareTestCLI();
});

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.sequential("compilation tests", async () => {
  test.sequential("the CallerAsync contract is compiled successfully", async () => {
    const { stdout, stderr } = await exec(CALLER_ASYNC_COMPILATION_COMMAND);
    expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
  });

  test.sequential("the CallerAsyncBasicPattern contract is compiled successfully", async () => {
    const { stdout, stderr } = await exec(CALLER_ASYNC_BP_COMPILATION_COMMAND);
    expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
  });

  test.sequential("the Escrow contract is compiled successfully", async () => {
    const { stdout, stderr } = await exec(ESCROW_COMPILATION_COMMAND);
    expect(stderr).toMatch(ESCROW_SUCCESSFUL_PATTERN);
  });

  test.sequential("the Validator contract is compiled successfully", async () => {
    const { stdout, stderr } = await exec(VALIDATOR_COMPILATION_COMMAND);
    expect(stderr).toMatch(RETAILER_COMPILATION_PATTERN);
  });
});

describe.sequential("Awaiter tests", async () => {
  test.sequential("compilation and deployment of Awaiter is successful", async () => {
    let { stdout, stderr } = await exec(AWAITER_COMPILATION_COMMAND);
    expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
    ({ stdout, stderr } = await exec(TEST_COMMANDS["AWAITER_DEPLOYMENT_COMMAND"]));
    expect(stdout).toMatch(CONTRACT_ADDRESS_PATTERN);
    const addressMatches = stdout.match(ADDRESS_PATTERN);
    AWAITER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;
    await exec(`${NIL_GLOBAL} wallet send-tokens ${AWAITER_ADDRESS} 5000000 ${CONFIG_FLAG}`);
  });

  test.sequential("Awaiter can call Counter successfully", async () => {
    await exec(COUNTER_COMPILATION_COMMAND);
    let { stdout, stderr } = await exec(TEST_COMMANDS["COUNTER_DEPLOYMENT_COMMAND"]);
    expect(stdout).toMatch(CONTRACT_ADDRESS_PATTERN);
    const addressMatches = stdout.match(ADDRESS_PATTERN);
    COUNTER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;

    await exec(
      `${NIL_GLOBAL} wallet send-message ${AWAITER_ADDRESS} call ${COUNTER_ADDRESS} --abi ./tests/Awaiter/Awaiter.abi ${CONFIG_FLAG}`,
    );

    ({ stdout, stderr } = await exec(
      `${NIL_GLOBAL} contract call-readonly ${AWAITER_ADDRESS} getResult --abi ./tests/Awaiter/Awaiter.abi ${CONFIG_FLAG}`,
    ));
    const normalize = (str) => str.replace(/\r\n/g, "\n").trim();

    const expectedOutput = "Success, result:\nuint256: 0";
    const receivedOutput = normalize(stdout);

    expect(receivedOutput).toBe(expectedOutput);
  });
});
