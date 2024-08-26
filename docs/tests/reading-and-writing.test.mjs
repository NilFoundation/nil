import { RPC_GLOBAL, NIL_GLOBAL } from './globals';

const RPC_ENDPOINT = RPC_GLOBAL;
const CONFIG_FILE_NAME = 'tempReadingAndWriting.ini';

const util = require('node:util');
const exec = util.promisify(require('node:child_process').exec);

const SALT = BigInt(Math.floor(Math.random() * 10000));

const CONFIG_FLAG = `--config ./tests/${CONFIG_FILE_NAME}`;

const CONFIG_COMMAND = `${NIL_GLOBAL} config init ${CONFIG_FLAG}`;
const RPC_COMMAND = `${NIL_GLOBAL} config set rpc_endpoint ${RPC_ENDPOINT} ${CONFIG_FLAG}`;
const KEYGEN_COMMAND = `${NIL_GLOBAL} keygen new ${CONFIG_FLAG}`;

//startWallet
const WALLET_CREATION_COMMAND = `${NIL_GLOBAL} wallet new --salt ${SALT} ${CONFIG_FLAG}`;
//endWallet

//startLatestBlock
const RETRIEVE_LATEST_BLOCK_COMMAND = `${NIL_GLOBAL} block latest --shard-id 1 ${CONFIG_FLAG}`;
//endLatestBlock

const COUNTER_COMPILATION_COMMAND = 'solc -o ./tests/Counter --bin --abi ./tests/Counter.sol --overwrite';
const COUNTER_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} wallet deploy ./tests/Counter/Counter.bin --salt ${SALT} ${CONFIG_FLAG}`;

let CONTRACT_ADDRESS;

beforeAll(async () => {
    await exec(CONFIG_COMMAND);
    await exec(KEYGEN_COMMAND);
    await exec(RPC_COMMAND);
    await exec(WALLET_CREATION_COMMAND);
});

afterAll(async () => {
    await exec(`rm -rf ./tests/${CONFIG_FILE_NAME}`);
});

describe.sequential('CLI tests', async () => {
    test.sequential('the CLI correctly retrieves the latest block', async () => {
        const PREV_BLOCK_PATTERN = /PrevBlock/;
        const { stdout, stderr } = await exec(RETRIEVE_LATEST_BLOCK_COMMAND);
        expect(stdout).toBeDefined;
        expect(stdout).toMatch(PREV_BLOCK_PATTERN);
    });

    test.sequential('the CLI can read messages and receipts', async () => {
        const hashPattern = /0x[a-fA-F0-9]{64}/g;
        await exec(COUNTER_COMPILATION_COMMAND);
        // biome-ignore lint/style/useConst: <explanation>
        let { stdout, stderr } = await exec(COUNTER_DEPLOYMENT_COMMAND);
        expect(stdout).toBeDefined;
        const HASH = stdout.match(hashPattern)[0];
        //startMessageRead
        const READ_MESSAGE_COMMAND = `${NIL_GLOBAL} message ${HASH} ${CONFIG_FLAG}`;
        //endMessageRead
        ({ stdout, stderr } = await exec(READ_MESSAGE_COMMAND));
        expect(stdout).toBeDefined;
        //startReceiptRead
        const READ_RECEIPT_COMMAND = `${NIL_GLOBAL} receipt ${HASH} ${CONFIG_FLAG}`;
        //endReceiptRead
        ({ stdout, stderr } = await exec(READ_RECEIPT_COMMAND));
        expect(stdout).toBeDefined;
    });
});
