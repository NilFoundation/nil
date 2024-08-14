const {
    Faucet,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    WalletV1,
    generateRandomPrivateKey,
    convertEthToWei
} = require("@nilfoundation/niljs");
import { expect, describe, test, it, beforeEach, testOnly } from "vitest";

const RPC_ENDPOINT = "https://api.devnet.nil.foundation/api/nil_user/TEK83KSDZH58AIK9PCYSNU4G86DU55I9/";

const util = require('util');
const exec = util.promisify(require('child_process').exec);

let SALT = BigInt(Math.floor(Math.random() * 10000));

const KEYGEN_COMMAND = 'nil keygen new';

//startWallet
let WALLET_CREATION_COMMAND = `nil wallet new --salt ${SALT}`;
//endWallet


//startBalance
const WALLET_BALANCE_COMMAND = 'nil wallet balance';
//endBalance


const WALLET_TOP_UP_COMMAND = 'nil wallet top-up 1000000';

describe.sequential('initial CLI tests', () => {
    test.sequential('wallet creation command creates a wallet', async () => {
        const pattern = /New wallet address/;
        await exec(KEYGEN_COMMAND);
        const { stdout, stderr } = await exec(WALLET_CREATION_COMMAND);
        expect(stdout).toMatch(pattern);
    }, 20000);

    test.sequential('wallet balance command returns balance', async () => {
        const pattern = /Wallet balance/;
        const { stdout, stderr } = await exec(WALLET_BALANCE_COMMAND);
        expect(stdout).toMatch(pattern);
    }, 20000);
});

describe.sequential('niljs test', () => {
    test.sequential('niljs snippet can create and deploy a wallet', async () => {
        //startNilJSWalletCreation
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
            salt: SALT,
            shardId: 1,
            client,
            signer,
        });

        const walletAddress = wallet.getAddressHex();

        await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(1));

        await wallet.selfDeploy(true);
        //endNilJSWalletCreation
        expect(walletAddress).toBeDefined();
        const walletCode = await client.getCode(walletAddress, "latest");
        expect(walletCode).toBeDefined();
        expect(walletCode.length).toBeGreaterThan(10);
    }, 20000);
});




