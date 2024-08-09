const RPC_ENDPOINT = "https://api.devnet.nil.foundation/api/nil_user/TEK83KSDZH58AIK9PCYSNU4G86DU55I9/";

const util = require('util');
const exec = util.promisify(require('child_process').exec);

let salt = BigInt(Math.floor(Math.random() * 10000));

//startKeygen
const KEYGEN_COMMAND = 'nil keygen new';
//endKeygen

//startEndpoint
const RPC_COMMAND = `nil config set rpc_endpoint '${RPC_ENDPOINT}'`;
//endEndpoint

//startWallet
const WALLET_CREATION_COMMAND = 'nil wallet new';
//endWallet

//startTopup
const WALLET_TOP_UP_COMMAND = 'nil wallet top-up 1000000';
//endTopup

//startInfo
const WALLET_INFO_COMMAND = 'nil wallet info';
//endInfo

//startCompilation
const COUNTER_COMPILATION_COMMAND = 'solc -o ./tests/Counter --bin --abi ./tests/Counter.sol --overwrite';
//endCompilation

//startDeploy
const COUNTER_DEPLOYMENT_COMMAND = `nil wallet deploy ./tests/Counter/Counter.bin --salt ${salt}`;
//endDeploy

let COUNTER_ADDRESS;

//startCallerCompilation
const CALLER_COMPILATION_COMMAND = 'solc -o ./tests/Caller --bin --abi ./tests/Caller.sol --overwrite';
//endCallerCompilation

//start_CallerDeploy
const CALLER_DEPLOYMENT_COMMAND = `nil wallet deploy ./tests/Caller/Caller.bin --shard-id 2 --salt ${salt}`;
//end_CallerDeploy

let CALLER_ADDRESS;

//startNewWalletDeploy
const NEW_WALLET_COMMAND = `nil wallet new --salt ${salt}`;
//endNewWalletDeploy

let NEW_WALLET_ADDRESS;

test('keygen generation works via CLI', async () => {
    const pattern = /\bPrivate key: [a-f0-9]{64}\b/;
    const { stdout, stderr } = await exec(KEYGEN_COMMAND);
    expect(stdout).toMatch(pattern);
});

test('endpoint command should set the endpoint', async () => {
    const pattern = /Set "rpc_endpoint" to /;
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
    COUNTER_ADDRESS = addressMatches.length > 1 ? addressMatches[1] : null;
}, 10000);

test('execution of increment produces a message', async () => {
    //startIncrement
    const COUNTER_INCREMENT_COMMAND = `nil wallet send-message ${COUNTER_ADDRESS} increment --abi ./tests/Counter/Counter.abi`;
    //endIncrement
    const pattern = /Message hash:/;
    const { stdout, stderr } = await exec(COUNTER_INCREMENT_COMMAND);
    expect(stdout).toMatch(pattern);

}, 10000);

test('call to incrementer returns the correct value', async () => {
    //start_CallToIncrementer
    const COUNTER_CALL_READONLY_COMMAND = `nil contract call-readonly ${COUNTER_ADDRESS} getValue --abi ./tests/Counter/Counter.abi`;
    //end_CallToIncrementer
    const { stdout, stderr } = await exec(COUNTER_CALL_READONLY_COMMAND);

    const normalize = str => str.replace(/\r\n/g, '\n').trim();

    const expectedOutput = "Success, result:\nuint256: 1";
    const receivedOutput = normalize(stdout);

    expect(receivedOutput).toBe(expectedOutput);
}, 10000);

test('deploy of caller works successfully', async () => {
    const pattern = /Contract address:/;
    const addressPattern = /Contract address:\s*(0x[a-fA-F0-9]{40})/;
    await exec(CALLER_COMPILATION_COMMAND);
    const { stdout, stderr } = await exec(CALLER_DEPLOYMENT_COMMAND);
    expect(stdout).toMatch(pattern);
    let addressMatches = stdout.match(addressPattern);
    CALLER_ADDRESS = addressMatches && addressMatches.length > 0 ? addressMatches[1] : null;
    expect(CALLER_ADDRESS).not.toBeNull();
}, 10000);

test('caller can call incrementer successfully', async () => {
    //start_SendTokensToCaller
    const SEND_TOKENS_COMMAND = `nil wallet send-tokens ${CALLER_ADDRESS} 300000`;
    //end_SendTokensToCaller

    //startMessageFromCallerToIncrementer
    const SEND_FROM_CALLER_COMMAND = `nil wallet send-message ${CALLER_ADDRESS} call ${COUNTER_ADDRESS} --abi ./tests/Caller/Caller.abi`;
    //endMessageFromCallerToIncrementer

    await exec(SEND_TOKENS_COMMAND);
    const pattern = /Message hash:/;
    const { stdout, stderr } = await exec(SEND_FROM_CALLER_COMMAND);
    expect(stdout).toMatch(pattern);
    
    const COUNTER_CALL_READONLY_COMMAND_POST_CALLER = `nil contract call-readonly ${COUNTER_ADDRESS} getValue --abi ./tests/Counter/Counter.abi`;

    let stdoutCall, stderrCall;

    try {
        for (let attempt = 0; attempt < 5; attempt++) {
            ({ stdout: stdoutCall, stderr: stderrCall } = await exec(COUNTER_CALL_READONLY_COMMAND_POST_CALLER));
            
            if (stdoutCall) {
                break;
            }

            console.log(`Attempt ${attempt + 1}: Retrying after a short delay...`);
            await new Promise(resolve => setTimeout(resolve, 1000));
        }

        if (!stdoutCall) {
            throw new Error("Failed to get output from the contract call after multiple attempts.");
        }

        const normalize = str => str.replace(/\r\n/g, '\n').trim();

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
}, 20000);

test('a new wallet is created successfully', async () => {
    const pattern = /New wallet address/;
    const { stdout, stderr } = await exec(NEW_WALLET_COMMAND);
    expect(stdout).toMatch(pattern);
    const addressPattern = /New wallet address:\s*(0x[a-fA-F0-9]{40})/;
    let addressMatches = stdout.match(addressPattern);
    NEW_WALLET_ADDRESS = addressMatches && addressMatches.length > 0 ? addressMatches[1] : null;
}, 10000);

test('a new currency is created and withdrawn successfully', async() => {
    const pattern = /50000/;

    //startMintWithdrawCurrency
    const MINT_WITHDRAW_CURRENCY_COMMAND = `nil minter create-currency ${NEW_WALLET_ADDRESS} 50000 new-currency --withdraw`;
    //endMintWithdrawCurrency
    
    await exec(MINT_WITHDRAW_CURRENCY_COMMAND);

    //startCurrenciesCheck
    const CURRENCIES_COMMAND = `nil contract currencies ${NEW_WALLET_ADDRESS}`;
    //endCurrenciesCheck

    const { stdout, stderr } = await exec(CURRENCIES_COMMAND);
    expect(stdout).toMatch(pattern);
    console.log(stdout);
}, 20000)