import { beforeAll, describe, expect, test } from "vitest";
const {
    Faucet,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    WalletV1,
    generateRandomPrivateKey,
    convertEthToWei,
    waitTillCompleted,
    MINTER_ABI,
    MINTER_ADDRESS,
    hexToBigInt,
    bytesToHex
} = require("@nilfoundation/niljs");

import { encodeFunctionData } from "viem";

const util = require('util');
const exec = util.promisify(require('child_process').exec);
const RPC_ENDPOINT = "https://api.devnet.nil.foundation/api/nil_user/TEK83KSDZH58AIK9PCYSNU4G86DU55I9/";

const NAME = 'newToken';
let SALT = BigInt(Math.floor(Math.random() * 10000));

const AMOUNT = 5000;
const WALLET_ADDRESS_PATTERN = /0x[a-fA-F0-9]{40}/;
const CREATED_CURRENCY_PATTERN = /Created Currency ID:/;
const CURRENCY_WITHDRAWN_PATTERN = /Withdraw 5000 amount of currency/;

const RPC_COMMAND = `nil config set rpc_endpoint ${RPC_ENDPOINT}`;
const KEYGEN_COMMAND = 'nil keygen new';
//startWalletCreation
const WALLET_CREATION_COMMAND = 'nil wallet new';
//endWalletCreation
const CURRENCIES_COMMAND = 'nil contract currencies';
//startSaltWalletCreation
const WALLET_CREATION_COMMAND_WITH_SALT = `nil wallet new --salt ${SALT}`;
//endSaltWalletCreation

let OWNER_ADDRESS;
let WALLET_ONE_ADDRESS;
let WALLET_TWO_ADDRESS;
beforeAll(async () => {
    await exec(RPC_COMMAND);
    await exec(KEYGEN_COMMAND);
    const { stdout, stderr } = await exec(WALLET_CREATION_COMMAND);
    OWNER_ADDRESS = stdout.match(WALLET_ADDRESS_PATTERN)[0];
});

describe.sequential('initial usage CLI tests', () => {
    test.sequential('minter creates a currency and withdraws it', async () => {

        //First
        const CREATE_CURRENCY_AND_WITHDRAW_COMMAND = `nil minter create-currency ${OWNER_ADDRESS} ${AMOUNT} ${NAME} --withdraw`;
        //Second

        const { stdout, stderr } = await exec(CREATE_CURRENCY_AND_WITHDRAW_COMMAND);
        expect(stdout).toMatch(CREATED_CURRENCY_PATTERN);
        const CURRENCIES_COMMAND_OWNER = CURRENCIES_COMMAND + ` ${OWNER_ADDRESS}`;
        const { stdoutResult, stderrResult } = await exec(CURRENCIES_COMMAND_OWNER);
        expect(stdoutResult).toBeDefined;
    }, 20000);

    test.sequential('minter creates a currency', async () => {

        //Third
        const CREATE_CURRENCY_COMMAND = `nil minter create-currency ${OWNER_ADDRESS} ${AMOUNT} ${NAME}`;
        //Fourth

        const { stdout, stderr } = await exec(CREATE_CURRENCY_COMMAND);
        expect(stdout).toMatch(CREATED_CURRENCY_PATTERN);
    }, 20000);
    test.sequential('minter withdraws an existing currency', async () => {
        const CREATE_CURRENCY_COMMAND = `nil minter create-currency ${OWNER_ADDRESS} 100000 newestToken`;
        await exec(CREATE_CURRENCY_COMMAND);

        //Fifth
        const WITHDRAW_EXISTING_CURRENCY_COMMAND = `nil minter withdraw-currency ${OWNER_ADDRESS} ${AMOUNT} ${OWNER_ADDRESS}`;
        //Sixth

        const { stdout, stderr } = await exec(WITHDRAW_EXISTING_CURRENCY_COMMAND);
        expect(stdout).toMatch(CURRENCY_WITHDRAWN_PATTERN);
    }, 20000);
});
describe.sequential('basic Nil.js usage tests', async () => {
    test.sequential('Nil.js can create a new currency, mint it, and withdraw it', async () => {

        //Seventh
        const client = new PublicClient({
            transport: new HttpTransport({
                endpoint: RPC_ENDPOINT,
            }),
            shardId: 1,
        });

        const faucet = new Faucet(client);

        const signer = new LocalECDSAKeySigner({
            privateKey: generateRandomPrivateKey(),
        });

        const pubkey = await signer.getPublicKey();
        const wallet = new WalletV1({
            pubkey: pubkey,
            salt: BigInt(Math.floor(Math.random() * 10000)),
            shardId: 1,
            client,
            signer,
        });

        const walletAddress = wallet.getAddressHex();

        const faucetHash = await faucet.withdrawToWithRetry(
            walletAddress,
            convertEthToWei(1),
        );

        await waitTillCompleted(client, 1, faucetHash);

        await wallet.selfDeploy(true);

        const hashMessage = await wallet.sendMessage({
            to: MINTER_ADDRESS,
            feeCredit: 5_000_000n,
            value: 1_000_000n,
            data: encodeFunctionData({
                abi: MINTER_ABI,
                functionName: "create",
                args: [1_000_000n, walletAddress, "MY_TOKEN", walletAddress],
            }),
        });
        await waitTillCompleted(client, 1, hashMessage);

        let tokens = await client.getCurrencies(walletAddress, "latest");
        //Eigth

        expect(tokens).toBeDefined;
        const previousAmount = tokens[Object.keys(tokens)[0]];

        //Ninth
        const hashMessageMint = await wallet.sendMessage({
            to: MINTER_ADDRESS,
            feeCredit: 5_000_000n,
            value: 1_000_000n,
            data: encodeFunctionData({
                abi: MINTER_ABI,
                functionName: "mint",
                args: [hexToBigInt(walletAddress), AMOUNT, walletAddress],
            }),
        });

        await waitTillCompleted(client, 1, hashMessageMint)
        //Tenth

        tokens = await client.getCurrencies(walletAddress, "latest");
        let newAmount = tokens[Object.keys(tokens)[0]];
        expect(newAmount > previousAmount).toBe(true);

        const hashMessageMintOther = await wallet.sendMessage({
            to: MINTER_ADDRESS,
            feeCredit: 5_000_000n,
            value: 1_000_000n,
            data: encodeFunctionData({
                abi: MINTER_ABI,
                functionName: "mint",
                args: [hexToBigInt(walletAddress), AMOUNT, walletAddress],
            }),
        });
        await waitTillCompleted(client, 1, hashMessageMintOther);
    }, 30000);
});
describe.sequential('tutorial flows CLI tests', async () => {
    test.sequential('the CLI creates two new wallets', async () => {
        await exec(KEYGEN_COMMAND);
        let { stdout, stderr } = await exec(WALLET_CREATION_COMMAND);
        expect(stdout).toMatch(WALLET_ADDRESS_PATTERN);
        WALLET_ONE_ADDRESS = stdout.match(WALLET_ADDRESS_PATTERN)[0];
        await exec(KEYGEN_COMMAND);
        ({ stdout, stderr } = await exec(WALLET_CREATION_COMMAND_WITH_SALT));
        expect(stdout).toMatch(WALLET_ADDRESS_PATTERN);
        WALLET_TWO_ADDRESS = stdout.match(WALLET_ADDRESS_PATTERN);
    }, 20000);
    test.sequential('the CLI creates new tokens and withdraws one of them', async () => {

        //Eleventh
        const FIRST_CURRENCY_CREATION_COMMAND = `nil minter create-currency ${WALLET_ONE_ADDRESS} 50000 token --withdraw`;
        //Twelfth

        //Thirteenth
        const SECOND_CURRENCY_CREATION_COMMAND = `nil minter create-currency ${WALLET_TWO_ADDRESS} 30000 newToken`;
        //Fourteenth

        console.log(WALLET_ONE_ADDRESS);
        let { stdout, stderr } = await exec(FIRST_CURRENCY_CREATION_COMMAND);
        expect(stdout).toMatch(CREATED_CURRENCY_PATTERN);
        ({ stdout, stderr } = await exec(SECOND_CURRENCY_CREATION_COMMAND));
        expect(stdout, stderr).toMatch(CREATED_CURRENCY_PATTERN);
        //Fifteenth
        const CURRENCIES_CHECK_COMMAND = `nil contract currencies ${WALLET_ONE_ADDRESS}`;
        //Sixteenth
        ({ stdout, stderr } = await exec(CURRENCIES_CHECK_COMMAND));
        console.log(stdout);
    }, 20000);
    test.sequential('the CLI withdraws a currency, and balances are updated correctly', async () => {

        //Eighteenth
        const SECOND_CURRENCY_WITHDRAWAL_COMMAND = `nil minter withdraw-currency ${WALLET_TWO_ADDRESS} 15000 ${WALLET_ONE_ADDRESS}`;
        //Nineteenth

        let { stdout, stderr } = await exec(SECOND_CURRENCY_WITHDRAWAL_COMMAND);
        expect(stdout).toBeDefined;
        const CURRENCIES_CHECK_COMMAND = `nil contract currencies ${WALLET_ONE_ADDRESS}`;
        ({ stdout, stderr } = await exec(CURRENCIES_CHECK_COMMAND));
        expect(stdout).toBeDefined;
        console.log(stdout);
        const FIRST_CURRENCY_PATTERN = /50000/;
        const SECOND_CURRENCY_PATTERN = /15000/;
        expect(stdout).toMatch(FIRST_CURRENCY_PATTERN);
        expect(stdout).toMatch(SECOND_CURRENCY_PATTERN);
    }, 20000);
});
describe.sequential('tutorial flows Nil.js tests', async () => {
    test('Nil.js successfully creates two wallets and handles currency transfers', async () => {

        //Twentieth
        const client = new PublicClient({
            transport: new HttpTransport({
                endpoint: RPC_ENDPOINT,
            }),
            shardId: 1,
        });

        const faucet = new Faucet(client);

        const signer = new LocalECDSAKeySigner({
            privateKey: generateRandomPrivateKey(),
        });

        const pubkey = await signer.getPublicKey();
        const wallet = new WalletV1({
            pubkey: pubkey,
            salt: BigInt(Math.floor(Math.random() * 10000)),
            shardId: 1,
            client,
            signer,
        });
        const walletAddress = wallet.getAddressHex();

        await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(1));
        await wallet.selfDeploy(true);

        const walletTwo = new WalletV1({
            pubkey: pubkey,
            salt: BigInt(Math.floor(Math.random() * 10000)),
            shardId: 1,
            client,
            signer,
        });
        const walletTwoAddress = walletTwo.getAddressHex();

        await faucet.withdrawToWithRetry(walletTwoAddress, convertEthToWei(1));
        await walletTwo.selfDeploy(true);
        //Twentyfirst

        //Twentysecond
        const currencyCreationMessage = await wallet.sendMessage({
            to: MINTER_ADDRESS,
            feeCredit: 5_000_000n,
            value: 1_000_000n,
            data: encodeFunctionData({
                abi: MINTER_ABI,
                functionName: "create",
                args: [10_000n, walletAddress, "token", walletAddress],
            }),
        });

        await waitTillCompleted(client, 1, currencyCreationMessage);
        //Twentythird

        //Twentyfourth
        const currencyCreationMessageTwo = await walletTwo.sendMessage({
            to: MINTER_ADDRESS,
            feeCredit: 5_000_000n,
            value: 1_000_000n,
            data: encodeFunctionData({
                abi: MINTER_ABI,
                functionName: "create",
                args: [20_000n, walletTwoAddress, "new-token", walletAddress],
            }),
        });

        await waitTillCompleted(client, 1, currencyCreationMessageTwo);
        //Twentyfifth

        //Twentysixth
        const tokens = await client.getCurrencies(walletAddress, "latest");
        //Twentyseventh
        expect(tokens[tokens.keys[0]]).toBe(10000n);
        expect(tokens[tokens.keys[0]]).toBe(20000n);
    }, 40000);
});