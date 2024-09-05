import { RPC_GLOBAL, NIL_GLOBAL, NODE_MODULES } from './globals';

const util = require('node:util');
const exec = util.promisify(require('node:child_process').exec);

// biome-ignore lint/style/useConst: <explanation>
let salt = BigInt(Math.floor(Math.random() * 10000));

const RPC_ENDPOINT = RPC_GLOBAL;
const CONFIG_FILE_NAME = 'tempWorkingWithSmartContracts.ini';

const CONFIG_FLAG = `--config ./tests/${CONFIG_FILE_NAME}`;
const CONFIG_COMMAND = `${NIL_GLOBAL} config init ${CONFIG_FLAG}`;
const WALLET_ADDRESS_PATTERN = /0x[a-fA-F0-9]{40}/;

//startKeygen
const KEYGEN_COMMAND = `${NIL_GLOBAL} keygen new ${CONFIG_FLAG}`;
//endKeygen

//startEndpoint
const RPC_COMMAND = `${NIL_GLOBAL} config set rpc_endpoint ${RPC_ENDPOINT} ${CONFIG_FLAG}`;
//endEndpoint

//startWalletCreationCommand
const WALLET_CREATION_COMMAND = `${NIL_GLOBAL} wallet new ${CONFIG_FLAG}`;
//endWalletCreationCommand

//startWalletInfoCommand
const WALLET_INFO_COMMAND = `${NIL_GLOBAL} wallet info ${CONFIG_FLAG}`;
//endWalletInfoCommand

//startRetailerCompilation
const RETAILER_COMPILATION_COMMAND = `solc -o ./tests/Retailer --bin --abi ./tests/Retailer.sol --overwrite ${NODE_MODULES}`;
//endRetailerCompilation

//startManufacturerCompilation
const MANUFACTURER_COMPILATION_COMMAND = `solc -o ./tests/Manufacturer --bin --abi ./tests/Manufacturer.sol --overwrite ${NODE_MODULES}`;
//endManufacturerCompilation

const SALT = BigInt(Math.floor(Math.random() * 10000));



let MANUFACTURER_ADDRESS;
let RETAILER_ADDRESS;
let PUBKEY;

beforeAll(async () => {
  await exec(CONFIG_COMMAND);
  await exec(KEYGEN_COMMAND);
  await exec(RPC_COMMAND);

});

afterAll(async () => {
  await exec(`rm -rf ./tests/${CONFIG_FILE_NAME}`);
});

describe.sequential('CLI deployment tests', async () => {
  test.sequential('compiling of Retailer and Manufacturer is successful', async () => {
    await exec(WALLET_CREATION_COMMAND);
    const RETAILER_COMPILATION_PATTERN = /Function state mutability can be restricted to pure/;
    const MANUFACTURER_COMPILATION_PATTERN = /Compiler run successful/;
    let { stdout, stderr } = await exec(RETAILER_COMPILATION_COMMAND);
    expect(stderr).toMatch(RETAILER_COMPILATION_PATTERN);
    ({ stdout, stderr } = await exec(MANUFACTURER_COMPILATION_COMMAND));
    expect(stdout).toMatch(MANUFACTURER_COMPILATION_PATTERN);
  });

  test.sequential('internal deployment of Retailer and Manufacturer is successful', async () => {
    const pattern = /Contract address:/;
    const addressPattern = /0x[a-fA-F0-9]{40}/g;
    //startRetailerDeploymentCommand
    const RETAILER_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} wallet deploy ./tests/Retailer/Retailer.bin --abi ./tests/Retailer/Retailer.abi --salt ${SALT} ${CONFIG_FLAG}`;
    //endRetailerDeploymentCommand
    let { stdout, stderr } = await exec(RETAILER_DEPLOYMENT_COMMAND);
    expect(stdout).toMatch(pattern);
    const addressMatches = stdout.match(addressPattern);
    RETAILER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;

    const pubkeyPattern = /Public key:\s(0x[a-fA-F0-9]+)/;
    ({ stdout, stderr } = await exec(WALLET_INFO_COMMAND));
    PUBKEY = stdout.match(pubkeyPattern)[1];

    //startManufacturerDeploymentCommand
    const MANUFACTURER_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} wallet deploy ./tests/Manufacturer/Manufacturer.bin ${PUBKEY} ${RETAILER_ADDRESS} --abi ./tests/Manufacturer/Manufacturer.abi --shard-id 2 --salt ${SALT} ${CONFIG_FLAG}`;
    //endManufacturerDeploymentCommand
    ({ stdout, stderr } = await exec(MANUFACTURER_DEPLOYMENT_COMMAND));
    const addressMatchesManufacturer = stdout.match(addressPattern);
    MANUFACTURER_ADDRESS = addressMatchesManufacturer.length > 1 ? addressMatchesManufacturer[1] : null;

  });
  test.sequential('internal deploy, the Retailer can call the Manufacturer successfully', async () => {
    //startSendTokensCommand
    const RETAILER_SEND_TOKENS_COMMAND = `${NIL_GLOBAL} wallet send-tokens ${RETAILER_ADDRESS} 5000000 ${CONFIG_FLAG}`;
    //endSendTokensCommand

    await exec(RETAILER_SEND_TOKENS_COMMAND);

    //startRetailerCallManufacturer
    const RETAILER_CALL_MANUFACTURER_COMMAND = `${NIL_GLOBAL} wallet send-message ${RETAILER_ADDRESS} orderProduct ${MANUFACTURER_ADDRESS} new-product --abi ./tests/Retailer/Retailer.abi --fee-credit 2000000 ${CONFIG_FLAG}`
    //endRetailerCallManufacturer

    let { stdout, stderr } = await exec(RETAILER_CALL_MANUFACTURER_COMMAND);
    expect(stdout).toBeDefined;

    await new Promise(resolve => setTimeout(resolve, 5000));

    //startCallToManufacturerCommand
    const CALL_TO_MANUFACTURER_COMMAND = `${NIL_GLOBAL} contract call-readonly ${MANUFACTURER_ADDRESS} getProducts --abi ./tests/Manufacturer/Manufacturer.abi ${CONFIG_FLAG}`;
    //endCallToManufacturerCommand

    ({ stdout, stderr } = await exec(CALL_TO_MANUFACTURER_COMMAND));
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(/new-product/);
  });

  test.sequential('external deployment of Retailer and Manufacturer is successful', async () => {
    const pattern = /Contract address:/;
    const addressPattern = /0x[a-fA-F0-9]{40}/g;
    salt = BigInt(Math.floor(Math.random() * 10000));

    //startExternalRetailerAddressCommand
    const RETAILER_ADDRESS_COMMAND = `${NIL_GLOBAL} contract address ./tests/Retailer.sol --shard-id 1 --salt ${salt} ${CONFIG_FLAG}`;
    //endExternalRetailerAddressCommand

    let { stdout, stderr } = await exec(RETAILER_ADDRESS_COMMAND);
    expect(stdout).toMatch(addressPattern);
    let addressMatches = stdout.match(addressPattern);
    RETAILER_ADDRESS = addressMatches[0];

    const AMOUNT = 10000000;

    //startSendTokensToRetailerForExternalDeploymentCommand
    const RETAILER_SEND_TOKENS_COMMAND_EXTERNAL = `${NIL_GLOBAL} wallet send-tokens ${RETAILER_ADDRESS} ${AMOUNT} ${CONFIG_FLAG}`;
    //endSendTokensToRetailerForExternalDeploymentCommand

    await exec(RETAILER_SEND_TOKENS_COMMAND_EXTERNAL);

    //startRetailerExternalDeploymentCommand
    const RETAILER_EXTERNAL_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} contract deploy ./tests/Retailer/Retailer.bin --shard-id 1 ${salt} ${CONFIG_FLAG}`;
    //endRetailerExternalDeploymentCommand

    ({ stdout, stderr } = await exec(RETAILER_EXTERNAL_DEPLOYMENT_COMMAND));
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(pattern);

    //startExternalManufacturerAddressCommand
    const MANUFACTURER_ADDRESS_COMMAND = `${NIL_GLOBAL} contract address ./tests/Manufacturer.sol ${PUBKEY} ${RETAILER_ADDRESS} --shard-id 2 --salt ${salt} ${CONFIG_FLAG} --abi ./tests/Manufacturer/Manufacturer.abi`;
    //endExternalManufacturerAddressCommand

    ({ stdout, stderr } = await exec(MANUFACTURER_ADDRESS_COMMAND));
    addressMatches = stdout.match(addressPattern);
    MANUFACTURER_ADDRESS = addressMatches[0];

    //startSendTokensToManufacturerForExternalDeploymentCommand
    const MANUFACTURER_SEND_TOKENS_COMMAND_EXTERNAL = `${NIL_GLOBAL} wallet send-tokens ${MANUFACTURER_ADDRESS} ${AMOUNT} ${CONFIG_FLAG}`;
    //endSendTokensToManufacturerForExternalDeploymentCommand

    await exec(MANUFACTURER_SEND_TOKENS_COMMAND_EXTERNAL);

    //startManufacturerExternalDeploymentCommand
    const MANUFACTURER_EXTERNAL_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} contract deploy ./tests/Manufacturer/Manufacturer.bin ${PUBKEY} ${RETAILER_ADDRESS} --salt ${SALT} --shard-id 2 --abi ./tests/Manufacturer/Manufacturer.abi ${CONFIG_FLAG}`;
    //endManufacturerExternalDeploymentCommand

    ({ stdout, stderr } = await exec(MANUFACTURER_EXTERNAL_DEPLOYMENT_COMMAND));
    expect(stdout).toBeDefined;
    expect(stdout).toMatch(pattern);
  });
});