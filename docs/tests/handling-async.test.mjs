import TestHelper from "./TestHelper";
import {
  CALLER_ASYNC_BP_COMPILATION_COMMAND,
  CALLER_ASYNC_COMPILATION_COMMAND,
  COUNTER_COMPILATION_COMMAND,
  ESCROW_COMPILATION_COMMAND,
  VALIDATOR_COMPILATION_COMMAND,
} from "./compilationCommands";
import { NIL_GLOBAL } from "./globals";
import {
  ADDRESS_PATTERN,
  CONTRACT_ADDRESS_PATTERN,
  ESCROW_SUCCESSFUL_PATTERN,
  RETAILER_COMPILATION_PATTERN,
  SUCCESSFUL_EXECUTION_PATTERN,
} from "./patterns";

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
