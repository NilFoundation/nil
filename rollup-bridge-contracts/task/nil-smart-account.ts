import { ethers } from 'ethers';
import * as fs from "fs";
import {
    convertEthToWei,
    FaucetClient,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    SmartAccountV1,
    Hex,
    waitTillCompleted,
} from "@nilfoundation/niljs";
import "dotenv/config";
import { L1NetworkConfig, L2NetworkConfig, loadL1NetworkConfig, loadNilNetworkConfig, saveL1NetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress } from '../scripts/utils/validate-config';

let smartAccount: SmartAccountV1 | null = null;

export async function loadNilSmartAccount(): Promise<SmartAccountV1> {
    const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;
    const faucetEndpoint = process.env.FAUCET_RPC_ENDPOINT as string;
    const client = new PublicClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });
    const faucetClient = new FaucetClient({
        transport: new HttpTransport({ endpoint: faucetEndpoint }),
    });

    const privateKey = process.env.NIL_PRIVATE_KEY as `0x${string}`;
    const config: L2NetworkConfig = loadNilNetworkConfig("local");
    let smartAccountAddress = config.l2CommonConfig.owner;

    const signer = new LocalECDSAKeySigner({ privateKey });
    smartAccount = new SmartAccountV1({
        signer,
        client,
        address: smartAccountAddress as `0x${string}`,
        pubkey: signer.getPublicKey(),
    });

    return smartAccount;
}

export async function loadL2DepositRecipientSmartAccount(): Promise<SmartAccountV1> {
    const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;
    const client = new PublicClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });

    const privateKey = process.env.DEPOSIT_RECIPIENT_PRIVATE_KEY as `0x${string}`;
    const config: L1NetworkConfig = loadL1NetworkConfig("geth");
    let smartAccountAddress = config.l1TestConfig.l2DepositRecipient;
    const signer = new LocalECDSAKeySigner({ privateKey });
    smartAccount = new SmartAccountV1({
        signer,
        client,
        address: smartAccountAddress as `0x${string}`,
        pubkey: signer.getPublicKey(),
    });

    return smartAccount;
}

export async function generateNilSmartAccount(networkName: string): Promise<[SmartAccountV1, SmartAccountV1]> {
    const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;
    const client = new PublicClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });

    const privateKey = process.env.NIL_PRIVATE_KEY as `0x${string}`;
    let smartAccountAddress = process.env.NIL_SMART_ACCOUNT_ADDRESS as `0x${string}`;

    if (privateKey && smartAccountAddress) {
        const signer = new LocalECDSAKeySigner({ privateKey });
        smartAccount = new SmartAccountV1({
            signer,
            client,
            address: smartAccountAddress,
            pubkey: signer.getPublicKey(),
        });
    } else {
        console.log(`creating new nil smart account`);
        const signer = new LocalECDSAKeySigner({ privateKey: privateKey });
        smartAccount = new SmartAccountV1({
            signer,
            client,
            salt: BigInt(Math.floor(Math.random() * 10000)),
            shardId: 1,
            pubkey: signer.getPublicKey(),
        });
        smartAccountAddress = smartAccount.address;
        fs.writeFileSync("smartAccount.json", JSON.stringify({
            PRIVATE_KEY: privateKey,
            SMART_ACCOUNT_ADDRESS: smartAccount.address,
        }));
        console.log("🆕 owner Smart Account Generated:", smartAccount.address);
    }

    const depositRecipientPrivateKey = process.env.DEPOSIT_RECIPIENT_PRIVATE_KEY as `0x${string}`;
    let signer = new LocalECDSAKeySigner({ privateKey: depositRecipientPrivateKey });
    const depositRecipientSmartAccount = new SmartAccountV1({
        signer,
        client,
        salt: BigInt(Math.floor(Math.random() * 10000)),
        shardId: 1,
        pubkey: signer.getPublicKey(),
    });
    const depositRecipientSmartAccountAddress = depositRecipientSmartAccount.address;
    console.log("🆕 depositRecipient Smart Account Generated:", depositRecipientSmartAccountAddress);


    const nilFeeRefundAddressPrivateKey = process.env.NIL_FEE_REFUND_PRIVATE_KEY as `0x${string}`;
    signer = new LocalECDSAKeySigner({ privateKey: nilFeeRefundAddressPrivateKey });
    const feeRefundSmartAccount = new SmartAccountV1({
        signer,
        client,
        salt: BigInt(Math.floor(Math.random() * 10000)),
        shardId: 1,
        pubkey: signer.getPublicKey(),
    });
    const feeRefundSmartAccountAddress = feeRefundSmartAccount.address;
    console.log("🆕 feeRefund Smart Account Generated:", feeRefundSmartAccountAddress);

    const faucetEndpoint = process.env.FAUCET_RPC_ENDPOINT as string;
    const faucetClient = new FaucetClient({
        transport: new HttpTransport({ endpoint: faucetEndpoint }),
    });

    console.log(`about to topup  owner via faucet`);
    const topUpFaucet = await faucetClient.topUp({
        smartAccountAddress: smartAccount.address,
        amount: convertEthToWei(0.1),
        faucetAddress: process.env.NIL as `0x${string}`,
    });

    console.log(`faucet topup initiation done`);

    await waitTillCompleted(client, topUpFaucet);

    if ((await smartAccount.checkDeploymentStatus()) === false) {
        await smartAccount.selfDeploy(true);
    }

    console.log("✅ Smart Account Funded (100 ETH)");

    // update 
    const config: L2NetworkConfig = loadNilNetworkConfig(networkName);

    config.l2CommonConfig.owner = getCheckSummedAddress(smartAccountAddress);
    config.l2CommonConfig.admin = getCheckSummedAddress(smartAccountAddress);

    const l1Config = loadL1NetworkConfig("geth");

    l1Config.l1TestConfig.l2DepositRecipient = getCheckSummedAddress(depositRecipientSmartAccountAddress);
    l1Config.l1TestConfig.l2FeeRefundRecipient = getCheckSummedAddress(feeRefundSmartAccountAddress);

    // Save the updated config
    saveNilNetworkConfig(networkName, config);
    saveL1NetworkConfig("geth", l1Config);

    return [smartAccount, depositRecipientSmartAccount];
}

export async function prepareNilSmartAccountsForUnitTest(): Promise<{
    ownerSmartAccount: SmartAccountV1,
    depositRecipientSmartAccount: SmartAccountV1,
    feeRefundSmartAccount: SmartAccountV1
}> {
    const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;
    const faucetEndpoint = process.env.FAUCET_ENDPOINT as string;

    const faucetClient = new FaucetClient({
        transport: new HttpTransport({ endpoint: faucetEndpoint }),
    });

    const client = new PublicClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });
    // Generate a new ECDSA key pair
    const owner_wallet = ethers.Wallet.createRandom();
    let owner_privateKey = owner_wallet.privateKey;
    let ownerSmartAccount: SmartAccountV1 | null = null;

    const owner_signer = new LocalECDSAKeySigner({ privateKey: owner_privateKey as `0x${string}` });

    try {
        ownerSmartAccount = new SmartAccountV1({
            signer: owner_signer,
            client: client,
            salt: BigInt(Math.floor(Math.random() * 10000)),
            shardId: 1,
            pubkey: owner_signer.getPublicKey(),
        });

        await topupSmartAccount(faucetClient, client, ownerSmartAccount.address);

        if ((await ownerSmartAccount.checkDeploymentStatus()) === false) {
            await ownerSmartAccount.selfDeploy(true);
        }

        console.log("🆕 owner Smart Account Generated:", ownerSmartAccount.address);
    } catch (err) {
        console.error(`failed to self-deploy owner-smart-account: ${err}`);
        return;
    }

    const deposit_recipient_wallet = ethers.Wallet.createRandom();
    const deposit_recipient_privateKey = deposit_recipient_wallet.privateKey; // This is a 0x... string
    const deposit_recipient_wallet_address = deposit_recipient_wallet.address;       // This is the public address

    let deposit_recipient_signer = new LocalECDSAKeySigner({ privateKey: deposit_recipient_privateKey as Hex });
    const depositRecipientSmartAccount = new SmartAccountV1({
        signer: deposit_recipient_signer,
        client,
        salt: BigInt(Math.floor(Math.random() * 10000)),
        shardId: 1,
        pubkey: deposit_recipient_signer.getPublicKey(),
    });
    const depositRecipientSmartAccountAddress = depositRecipientSmartAccount.address;

    await topupSmartAccount(faucetClient, client, depositRecipientSmartAccountAddress);

    if ((await depositRecipientSmartAccount.checkDeploymentStatus()) === false) {
        await depositRecipientSmartAccount.selfDeploy(true);
    }

    console.log("🆕 depositRecipient Smart Account Generated:", depositRecipientSmartAccountAddress);

    const nil_refund_wallet = ethers.Wallet.createRandom();
    const nil_refund_privateKey = nil_refund_wallet.privateKey; // This is a 0x... string

    const nil_refund_signer = new LocalECDSAKeySigner({ privateKey: nil_refund_privateKey as Hex });
    const feeRefundSmartAccount = new SmartAccountV1({
        signer: nil_refund_signer,
        client,
        salt: BigInt(Math.floor(Math.random() * 10000)),
        shardId: 1,
        pubkey: nil_refund_signer.getPublicKey(),
    });
    const feeRefundSmartAccountAddress = feeRefundSmartAccount.address;
    await topupSmartAccount(faucetClient, client, feeRefundSmartAccountAddress);

    if ((await feeRefundSmartAccount.checkDeploymentStatus()) === false) {
        await feeRefundSmartAccount.selfDeploy(true);
    }

    console.log("🆕 feeRefund Smart Account Generated:", feeRefundSmartAccountAddress);

    return {
        ownerSmartAccount,
        depositRecipientSmartAccount,
        feeRefundSmartAccount
    };
}

export async function topupSmartAccount(faucetClient: FaucetClient, client: PublicClient, smartAccountAddress: String) {
    const topUpFaucet = await faucetClient.topUp({
        smartAccountAddress: smartAccountAddress as Hex,
        amount: convertEthToWei(0.1),
        faucetAddress: process.env.NIL as `0x${string}`,
    });
    await waitTillCompleted(client, topUpFaucet);
}

