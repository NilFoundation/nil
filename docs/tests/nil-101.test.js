const RPC_ENDPOINT = "http://127.0.0.1:8529";

const util = require('util');
const exec = util.promisify(require('child_process').exec);

let salt = BigInt(Math.floor(Math.random() * 10000));


const KEYGEN_COMMAND = '../build/bin/nil_cli keygen new';

const RPC_COMMAND = `../build/bin/nil_cli config set rpc_endpoint '${RPC_ENDPOINT}'`;
const WALLET_CREATION_COMMAND = '../build/bin/nil_cli wallet new';
const WALLET_TOP_UP_COMMAND = '../build/bin/nil_cli wallet top-up 1000000';
const WALLET_INFO_COMMAND = '../build/bin/nil_cli wallet info';
const COUNTER_COMPILATION_COMMAND = 'solc -o ./tests/Counter --bin --abi ./tests/counter_docs.sol --overwrite';
const COUNTER_DEPLOYMENT_COMMAND = `../build/bin/nil_cli wallet deploy ./tests/Counter/Incrementer.bin --salt ${salt}`;
let incrementerAddress;



test('keygen generation works via CLI', async () => {
    const pattern = /\bPrivate key: [a-f0-9]{64}\b/;
    const { stdout, stderr } = await exec(KEYGEN_COMMAND);
    expect(stdout).toMatch(pattern);
});

test('endpoint command should set the endpoint', async () => {
    const pattern = /Set "rpc_endpoint" to "http:\/\/127\.0\.0\.1:8529"/;
    const { stdout, stderr } = await exec(RPC_COMMAND);
    expect(stderr).toMatch(pattern);
});

test('wallet creation command creates a wallet', async () => {
    const pattern = /New wallet address/;
    const { stdout, stderr } = await exec(WALLET_CREATION_COMMAND);
    expect(stdout).toMatch(pattern);
}, 10000);

test('wallet top-up command tops up the wallet', async () => {
    const pattern = /Wallet balance/;
    const { stdout, stderr } = await exec(WALLET_TOP_UP_COMMAND);
    expect(stdout).toMatch(pattern);
}, 10000);

test('wallet info command supplies info', async () => {
    const pattern = /Wallet address/;
    const { stdout, stderr } = await exec(WALLET_INFO_COMMAND);
    expect(stdout).toMatch(pattern);
});

test('deploy of incrementer works successfully', async () => {
    const pattern = /Contract address:/;
    const addressPattern = /0x[a-fA-F0-9]{40}/g;
    await exec(COUNTER_COMPILATION_COMMAND);
    const { stdout, stderr } = await exec(COUNTER_DEPLOYMENT_COMMAND);
    expect(stdout).toMatch(pattern);
    let addressMatches = stdout.match(addressPattern);
    incrementerAddress = addressMatches.length > 1 ? addressMatches[1] : null;
}, 10000);

test('execution of increment produces a message', async () => {
    const COUNTER_INCREMENT_COMMAND = `../build/bin/nil_cli wallet send-message ${incrementerAddress} increment --abi ./tests/Counter/Incrementer.abi`;
    const pattern = /Message hash:/;
    const { stdout, stderr } = await exec(COUNTER_INCREMENT_COMMAND);
    expect(stdout).toMatch(pattern);
}, 10000);

test('call to incrementer returns the correct value', async () => {
    const COUNTER_CALL_READONLY_COMMAND = `../build/bin/nil_cli contract call-readonly ${incrementerAddress} getValue --abi ./tests/Counter/Incrementer.abi`;
    const { stdout, stderr } = await exec(COUNTER_CALL_READONLY_COMMAND);

    const normalize = str => str.replace(/\r\n/g, '\n').trim();

    const expectedOutput = "Success, result:\nuint256: 1";
    const receivedOutput = normalize(stdout);

    expect(receivedOutput).toBe(expectedOutput);
}, 10000);


