import { RPC_GLOBAL, NIL_GLOBAL } from './globals';
const SALT = BigInt(Math.floor(Math.random() * 10000));

const RPC_ENDPOINT = RPC_GLOBAL;
const CONFIG_FILE_NAME = 'tempConfigAsyncTests.ini';

const CONFIG_FLAG = `--config ./tests/${CONFIG_FILE_NAME}`;
const CONFIG_COMMAND = `${NIL_GLOBAL} config init ${CONFIG_FLAG}`;


const KEYGEN_COMMAND = `${NIL_GLOBAL} keygen new ${CONFIG_FLAG}`;

const RPC_COMMAND = `${NIL_GLOBAL} config set rpc_endpoint ${RPC_ENDPOINT} ${CONFIG_FLAG}`;

const WALLET_CREATION_COMMAND = `${NIL_GLOBAL} wallet new ${CONFIG_FLAG} --salt ${SALT}`;

const COUNTER_COMPILATION_COMMAND = 'solc -o ./tests/Counter --bin --abi ./tests/Counter.sol --overwrite';

const COUNTER_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} wallet deploy ./tests/Counter/Counter.bin --salt ${SALT} ${CONFIG_FLAG}`;

const util = require('node:util');
const exec = util.promisify(require('node:child_process').exec);

const SUCCESSFUL_EXECUTION_PATTERN = /Compiler run successful/;

const pattern = /Contract address:/;
const addressPattern = /0x[a-fA-F0-9]{40}/g;

let COUNTER_ADDRESS;
let AWAITER_ADDRESS;


beforeAll(async () => {
    await exec(CONFIG_COMMAND);
    await exec(KEYGEN_COMMAND);
    await exec(RPC_COMMAND);
    await exec(WALLET_CREATION_COMMAND);
});

afterAll(async () => {
    await exec(`rm -rf ./tests/${CONFIG_FILE_NAME}`);
});

describe.sequential('compilation tests', async () => {
    test.sequential('the CallerAsync contract is compiled successfully', async () => {
        const CALLER_ASYNC_COMPILATION_COMMAND = 'solc -o ./tests/CallerAsync --bin --abi ./tests/CallerAsync.sol --overwrite';
        const { stdout, stderr } = await exec(CALLER_ASYNC_COMPILATION_COMMAND);
        expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
    });

    test.sequential('the CallerAsyncBasicPattern contract is compiled successfully', async () => {
        const CALLER_ASYNC_BP_COMPILATION_COMMAND = 'solc -o ./tests/CallerAsyncBasicPattern --bin --abi ./tests/CallerAsyncBasicPattern.sol  --overwrite';
        const { stdout, stderr } = await exec(CALLER_ASYNC_BP_COMPILATION_COMMAND);
        expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
    });

    test.sequential('the Escrow contract is compiled successfully', async () => {
        const ESCROW_SUCCESSFUL_PATTERN = /Function state mutability can be restricted to pure/;
        const ESCROW_COMPILATION_COMMAND = 'solc -o ./tests/Escrow --bin --abi ./tests/Escrow.sol  --overwrite';
        const { stdout, stderr } = await exec(ESCROW_COMPILATION_COMMAND);
        expect(stderr).toMatch(ESCROW_SUCCESSFUL_PATTERN);
    });

    test.sequential('the Validator contract is compiled successfully', async () => {
        const VALIDATOR_COMPILATION_COMMAND = 'solc -o ./tests/Validator --bin --abi ./tests/Validator.sol  --overwrite';
        const { stdout, stderr } = await exec(VALIDATOR_COMPILATION_COMMAND);
        expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
    });
});

describe.sequential('Awaiter tests', async () => {
    test.skip.sequential('compilation and deployment of Awaiter is successful', async () => {
        const AWAITER_COMPILATION_COMMAND = 'solc -o ./tests/Awaiter --bin --abi ./tests/Awaiter.sol --overwrite';
        let { stdout, stderr } = await exec(AWAITER_COMPILATION_COMMAND);
        expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
        ({ stdout, stderr } = await exec(`${NIL_GLOBAL} wallet deploy ./tests/Awaiter/Awaiter.bin --abi ./tests/Awaiter/Awaiter.abi --shard-id 2 ${CONFIG_FLAG} --salt ${SALT}`));
        expect(stdout).toMatch(pattern);
        const addressMatches = stdout.match(addressPattern);
        AWAITER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;
        await exec(`${NIL_GLOBAL} wallet send-tokens ${AWAITER_ADDRESS} 50000000`);
    });

    test.skip.sequential('Awaiter can call Counter successfully', async () => {

        await exec(COUNTER_COMPILATION_COMMAND);
        let { stdout, stderr } = await exec(COUNTER_DEPLOYMENT_COMMAND);
        expect(stdout).toMatch(pattern);
        const addressMatches = stdout.match(addressPattern);
        COUNTER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;

        await exec(`${NIL_GLOBAL} wallet send-message ${AWAITER_ADDRESS} call ${COUNTER_ADDRESS} --abi ./tests/Awaiter/Awaiter.abi ${CONFIG_FLAG}`);

        ({ stdout, stderr } = await exec(`${NIL_GLOBAL} contract call-readonly ${AWAITER_ADDRESS} getResult --abi ./tests/Awaiter/Awaiter.abi ${CONFIG_FLAG}`));
        const normalize = str => str.replace(/\r\n/g, '\n').trim();

        const expectedOutput = "Success, result:\nuint256: 0";
        const receivedOutput = normalize(stdout);

        expect(receivedOutput).toBe(expectedOutput);
    });
});





