import TestHelper from "./TestHelper";
import {
  MANUFACTURER_COMPILATION_COMMAND,
  RETAILER_COMPILATION_COMMAND,
} from "./compilationCommands";
import { NIL_GLOBAL } from "./globals";
import {
  ADDRESS_PATTERN,
  CONTRACT_ADDRESS_PATTERN,
  MANUFACTURER_COMPILATION_PATTERN,
  PUBKEY_PATTERN,
  RETAILER_COMPILATION_PATTERN,
} from "./patterns";
const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

const CONFIG_FILE_NAME = "./tests/tempWorkingWithSmartContracts.ini";

const CONFIG_FLAG = `--config ${CONFIG_FILE_NAME}`;

let MANUFACTURER_ADDRESS;
let RETAILER_ADDRESS;
let PUBKEY;
let testHelper;

beforeAll(async () => {
  testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  await testHelper.prepareTestCLI();
});

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.sequential("CLI deployment tests", async () => {
  test.sequential("compiling of Retailer and Manufacturer is successful", async () => {
    let { stdout, stderr } = await exec(RETAILER_COMPILATION_COMMAND);
    expect(stderr).toMatch(RETAILER_COMPILATION_PATTERN);
    ({ stdout, stderr } = await exec(MANUFACTURER_COMPILATION_COMMAND));
    expect(stdout).toMatch(MANUFACTURER_COMPILATION_PATTERN);
  });

  test.sequential("internal deployment of Retailer and Manufacturer is successful", async () => {
    const SALT = BigInt(Math.floor(Math.random() * 10000));
    //startRetailerDeploymentCommand
    const RETAILER_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} smart-account deploy ./tests/Retailer/Retailer.bin --abi ./tests/Retailer/Retailer.abi --salt ${SALT} ${CONFIG_FLAG}`;
    //endRetailerDeploymentCommand
    let { stdout, stderr } = await exec(RETAILER_DEPLOYMENT_COMMAND);
    expect(stdout).toMatch(CONTRACT_ADDRESS_PATTERN);
    const addressMatches = stdout.match(ADDRESS_PATTERN);
    RETAILER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;

    const TEST_COMMANDS = testHelper.createCLICommandsMap(SALT);
    ({ stdout, stderr } = await exec(TEST_COMMANDS.SMART_ACCOUNT_INFO_COMMAND));
    PUBKEY = stdout.match(PUBKEY_PATTERN)[1];

    //startManufacturerDeploymentCommand
    const MANUFACTURER_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} smart-account deploy ./tests/Manufacturer/Manufacturer.bin ${PUBKEY} ${RETAILER_ADDRESS} --abi ./tests/Manufacturer/Manufacturer.abi --shard-id 2 --salt ${SALT} ${CONFIG_FLAG}`;
    //endManufacturerDeploymentCommand

    ({ stdout, stderr } = await exec(MANUFACTURER_DEPLOYMENT_COMMAND));
    const addressMatchesManufacturer = stdout.match(ADDRESS_PATTERN);
    MANUFACTURER_ADDRESS =
      addressMatchesManufacturer.length > 1 ? addressMatchesManufacturer[1] : null;
  });
  test.sequential(
    "internal deploy, the Retailer can call the Manufacturer successfully",
    async () => {
      //startSendTokensCommand
      const RETAILER_SEND_TOKENS_COMMAND = `${NIL_GLOBAL} smart-account send-tokens ${RETAILER_ADDRESS} 5000000 ${CONFIG_FLAG}`;
      //endSendTokensCommand

      await exec(RETAILER_SEND_TOKENS_COMMAND);

      const gasPrice = 20_000_000n;
      const feeCredit = 200_000n * gasPrice;

      //startRetailerCallManufacturer
      const RETAILER_CALL_MANUFACTURER_COMMAND = `${NIL_GLOBAL} smart-account send-transaction ${RETAILER_ADDRESS} orderProduct ${MANUFACTURER_ADDRESS} new-product --abi ./tests/Retailer/Retailer.abi --fee-credit ${feeCredit} ${CONFIG_FLAG}`;
      //endRetailerCallManufacturer

      let { stdout, stderr } = await exec(RETAILER_CALL_MANUFACTURER_COMMAND);
      expect(stdout).toBeDefined;

      await new Promise((resolve) => setTimeout(resolve, 5000));

      //startCallToManufacturerCommand
      const CALL_TO_MANUFACTURER_COMMAND = `${NIL_GLOBAL} contract call-readonly ${MANUFACTURER_ADDRESS} getProducts --abi ./tests/Manufacturer/Manufacturer.abi ${CONFIG_FLAG}`;
      //endCallToManufacturerCommand

      ({ stdout, stderr } = await exec(CALL_TO_MANUFACTURER_COMMAND));
      expect(stdout).toBeDefined;
      expect(stdout).toMatch(/new-product/);
    },
  );

  test.sequential("external deployment of Retailer and Manufacturer is successful", async () => {
    const SALT = BigInt(Math.floor(Math.random() * 10000));
    //startExternalRetailerAddressCommand
    const RETAILER_ADDRESS_COMMAND = `${NIL_GLOBAL} contract address ./tests/Retailer/Retailer.bin --shard-id 1 --salt ${SALT} ${CONFIG_FLAG}`;
    //endExternalRetailerAddressCommand

    let { stdout, stderr } = await exec(RETAILER_ADDRESS_COMMAND);
    expect(stdout).toMatch(ADDRESS_PATTERN);
    let addressMatches = stdout.match(ADDRESS_PATTERN);
    RETAILER_ADDRESS = addressMatches[0];

    const AMOUNT = 10000000;

    //startSendTokensToRetailerForExternalDeploymentCommand
    const RETAILER_SEND_TOKENS_COMMAND_EXTERNAL = `${NIL_GLOBAL} smart-account send-tokens ${RETAILER_ADDRESS} ${AMOUNT} ${CONFIG_FLAG}`;
    //endSendTokensToRetailerForExternalDeploymentCommand

    await exec(RETAILER_SEND_TOKENS_COMMAND_EXTERNAL);

    //startRetailerExternalDeploymentCommand
    const RETAILER_EXTERNAL_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} contract deploy ./tests/Retailer/Retailer.bin --shard-id 1 --salt ${SALT} ${CONFIG_FLAG}`;
    //endRetailerExternalDeploymentCommand

    ({ stdout, stderr } = await exec(RETAILER_EXTERNAL_DEPLOYMENT_COMMAND));
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(CONTRACT_ADDRESS_PATTERN);

    //startExternalManufacturerAddressCommand
    const MANUFACTURER_ADDRESS_COMMAND = `${NIL_GLOBAL} contract address ./tests/Manufacturer/Manufacturer.bin ${PUBKEY} ${RETAILER_ADDRESS} --shard-id 2 --salt ${SALT} ${CONFIG_FLAG} --abi ./tests/Manufacturer/Manufacturer.abi`;
    //endExternalManufacturerAddressCommand

    ({ stdout, stderr } = await exec(MANUFACTURER_ADDRESS_COMMAND));
    addressMatches = stdout.match(ADDRESS_PATTERN);
    MANUFACTURER_ADDRESS = addressMatches[0];

    //startSendTokensToManufacturerForExternalDeploymentCommand
    const MANUFACTURER_SEND_TOKENS_COMMAND_EXTERNAL = `${NIL_GLOBAL} smart-account send-tokens ${MANUFACTURER_ADDRESS} ${AMOUNT} ${CONFIG_FLAG}`;
    //endSendTokensToManufacturerForExternalDeploymentCommand

    await exec(MANUFACTURER_SEND_TOKENS_COMMAND_EXTERNAL);

    //startManufacturerExternalDeploymentCommand
    const MANUFACTURER_EXTERNAL_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} contract deploy ./tests/Manufacturer/Manufacturer.bin ${PUBKEY} ${RETAILER_ADDRESS} --salt ${SALT} --shard-id 2 --abi ./tests/Manufacturer/Manufacturer.abi ${CONFIG_FLAG}`;
    //endManufacturerExternalDeploymentCommand

    ({ stdout, stderr } = await exec(MANUFACTURER_EXTERNAL_DEPLOYMENT_COMMAND));
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(CONTRACT_ADDRESS_PATTERN);
  });
});
