import { RPC_GLOBAL, NIL_GLOBAL } from './globals';
import {
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
} from "@nilfoundation/niljs";

import { encodeFunctionData } from "viem";

const util = require('node:util');
const exec = util.promisify(require('node:child_process').exec);

const RPC_ENDPOINT = RPC_GLOBAL
const CONFIG_FILE_NAME = 'tempConfigTokensMCCSupport.ini';

const NAME = 'newToken';
const SALT = BigInt(Math.floor(Math.random() * 10000));

const AMOUNT = 5000;
const WALLET_ADDRESS_PATTERN = /0x[a-fA-F0-9]{40}/;
const CREATED_CURRENCY_PATTERN = /Created Currency ID:\s(0x[a-fA-F0-9]+)/;

const CONFIG_FLAG = `--config ./tests/${CONFIG_FILE_NAME}`;

const CONFIG_COMMAND = `${NIL_GLOBAL} config init ${CONFIG_FLAG}`;
const RPC_COMMAND = `${NIL_GLOBAL} config set rpc_endpoint ${RPC_ENDPOINT} ${CONFIG_FLAG}`;
const KEYGEN_COMMAND = `${NIL_GLOBAL} keygen new ${CONFIG_FLAG}`;
//startWalletCreation
const WALLET_CREATION_COMMAND = `${NIL_GLOBAL} wallet new ${CONFIG_FLAG}`;
//endWalletCreation
const CURRENCIES_COMMAND = `${NIL_GLOBAL} contract currencies ${CONFIG_FLAG}`;
//startSaltWalletCreation
const WALLET_CREATION_COMMAND_WITH_SALT = `${NIL_GLOBAL} wallet new --salt ${SALT} ${CONFIG_FLAG}`;
//endSaltWalletCreation

const COUNTER_COMPILATION_COMMAND = 'solc -o ./tests/Counter --bin --abi ./tests/Counter.sol --overwrite';
const COUNTER_DEPLOYMENT_COMMAND = `${NIL_GLOBAL} wallet deploy ./tests/Counter/Counter.bin --salt ${SALT} ${CONFIG_FLAG}`;


let OWNER_ADDRESS;
let WALLET_ADDRESS;
let CONTRACT_ADDRESS;

beforeAll(async () => {
    await exec(CONFIG_COMMAND);
    await exec(KEYGEN_COMMAND);
    await exec(RPC_COMMAND);
    const { stdout, stderr } = await exec(WALLET_CREATION_COMMAND);
    OWNER_ADDRESS = stdout.match(WALLET_ADDRESS_PATTERN)[0];
});

afterAll(async () => {
    await exec(`rm -rf ./tests/${CONFIG_FILE_NAME}`);
});

describe.sequential('initial usage CLI tests', () => {
    test.sequential('minter creates a currency and withdraws it', async () => {
        //startBasicCreateCurrencyCommand
        const CREATE_CURRENCY_COMMAND = `${NIL_GLOBAL} minter create-currency ${OWNER_ADDRESS} ${AMOUNT} ${NAME} ${CONFIG_FLAG}`;
        //endBasicCreateCurrencyCommand
        //startBasicWithdrawCurrencyCommand
        const BASIC_WITHDRAW_CURRENCY_COMMAND = `${NIL_GLOBAL} minter withdraw-currency ${OWNER_ADDRESS} ${AMOUNT} ${OWNER_ADDRESS} ${CONFIG_FLAG}`;
        //endBasicWithdrawCurrencyCommand
        let { stdout, stderr } = await exec(CREATE_CURRENCY_COMMAND);
        expect(stdout).toMatch(CREATED_CURRENCY_PATTERN);
        await exec(BASIC_WITHDRAW_CURRENCY_COMMAND);
        const CURRENCIES_COMMAND_OWNER = `${CURRENCIES_COMMAND} ${OWNER_ADDRESS} ${CONFIG_FLAG}`;
        ({ stdout, stderr } = await exec(CURRENCIES_COMMAND_OWNER));
        expect(stdout).toBeDefined();
        const CURRENCY_PATTERN = /5000/;
        expect(stdout).toMatch(CURRENCY_PATTERN);
    });
});
describe.skip.sequential('basic Nil.js usage tests', async () => {
    test.sequential('Nil.js can create a new currency, mint it, and withdraw it', async () => {

        //startBasicNilJSExample
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
            convertEthToWei(0.1)
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
        //endBasicNilJSExample

        expect(tokens).toBeDefined;
        const previousAmount = tokens[Object.keys(tokens)[0]];

        //startNilJSMintingExample
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
        //endNilJSMintingExample

        tokens = await client.getCurrencies(walletAddress, "latest");
        const newAmount = tokens[Object.keys(tokens)[0]];
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
    });
});
describe.sequential('tutorial flows CLI tests', async () => {
    test.sequential('the CLI creates a new wallet', async () => {
        await exec(KEYGEN_COMMAND);
        const { stdout, stderr } = await exec(WALLET_CREATION_COMMAND_WITH_SALT);
        expect(stdout).toMatch(WALLET_ADDRESS_PATTERN);
        WALLET_ADDRESS = stdout.match(WALLET_ADDRESS_PATTERN)[0];
        console.log(WALLET_ADDRESS);

    });
    test.sequential('the CLI creates new tokens', async () => {
        const addressPattern = /0x[a-fA-F0-9]{40}/g;
        //startCurrencyOneCreationCommand
        const CURRENCY_ONE_CREATION_COMMAND = `${NIL_GLOBAL} minter create-currency ${WALLET_ADDRESS} 50000 customToken ${CONFIG_FLAG}`;
        //endCurrencyOneCreationCommand]

        let { stdout, stderr } = await exec(CURRENCY_ONE_CREATION_COMMAND);
        expect(stdout).toBeDefined;
        expect(stdout).toMatch(CREATED_CURRENCY_PATTERN);

        //startCurrencyOneWithdrawalCommand
        const CURRENCY_ONE_WITHDRAWAL_COMMAND = `${NIL_GLOBAL} minter withdraw-currency ${WALLET_ADDRESS} 50000 ${WALLET_ADDRESS} ${CONFIG_FLAG}`;
        //endCurrencyOneWithdrawalCommand

        await exec(CURRENCY_ONE_WITHDRAWAL_COMMAND);

        await exec(COUNTER_COMPILATION_COMMAND);
        ({ stdout, stderr } = await exec(COUNTER_DEPLOYMENT_COMMAND));
        CONTRACT_ADDRESS = stdout.match(addressPattern)[1];

        //startCurrencyTwoCreationCommand
        const CURRENCY_TWO_CREATION_COMMAND = `${NIL_GLOBAL} minter create-currency ${CONTRACT_ADDRESS} 30000 newToken ${CONFIG_FLAG}`;
        //endCurrencyTwoCreationCommand

        ({ stdout, stderr } = await exec(CURRENCY_TWO_CREATION_COMMAND));
        expect(stdout).toBeDefined;
        expect(stdout).toMatch(CREATED_CURRENCY_PATTERN);
        console.log(stdout);

        //startCurrencyTwoWithdrawalCommand
        const CURRENCY_TWO_WITHDRAWAL_COMMAND = `${NIL_GLOBAL} minter withdraw-currency ${CONTRACT_ADDRESS} 30000 ${WALLET_ADDRESS} ${CONFIG_FLAG}`;
        //endCurrencyTwoWithdrawalCommand

        await exec(CURRENCY_ONE_WITHDRAWAL_COMMAND);
        ({ stdout, stderr } = await exec(CURRENCY_TWO_WITHDRAWAL_COMMAND));
        console.log(stdout);

        //startWalletCurrenciesCommand
        const WALLET_CURRENCIES_COMMAND = `${NIL_GLOBAL} contract currencies ${WALLET_ADDRESS}`;
        //endWalletCurrenciesCommand

        ({ stdout, stderr } = await exec(WALLET_CURRENCIES_COMMAND));
        console.log(stdout);
        expect(stdout).toMatch(/50000/);
        expect(stdout).toMatch(/30000/);
    }, 40000);
});
describe.skip.sequential('tutorial flows Nil.js tests', async () => {
    test('Nil.js successfully creates two wallets and handles currency transfers', async () => {

        //startAdvancedNilJSExample
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

        await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(0.1));
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
        //endAdvancedNilJSExample

        //startNilJSCreationTutorialAdvanced
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
        //endNilJSCreationTutorialAdvanced

        //startNilJSCreationTutorialWalletTwo
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
        //endNilJSCreationTutorialWalletTwo

        //startNilJSBalancesCheck
        const tokens = await client.getCurrencies(walletAddress, "latest");
        //endNilJSBalancesCheck
        expect(tokens[tokens.keys[0]]).toBe(10000n);
        expect(tokens[tokens.keys[0]]).toBe(20000n);
    });
});