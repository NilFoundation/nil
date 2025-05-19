import { ethers } from 'ethers';
import * as fs from "fs";
import {
    convertEthToWei,
    FaucetClient,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    SmartAccountV1,
    waitTillCompleted,
} from "@nilfoundation/niljs";
import "dotenv/config";
import { L1NetworkConfig, L2NetworkConfig, loadL1NetworkConfig, loadNilNetworkConfig, saveL1NetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress } from '../scripts/utils/validate-config';

let smartAccount: SmartAccountV1 | null = null;

export async function loadNilSmartAccount(): Promise<SmartAccountV1> {
    const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;
    const client = new PublicClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });
    const faucetClient = new FaucetClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
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

export async function generateNilSmartAccount(networkName: string): Promise<SmartAccountV1> {
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

    const faucetClient = new FaucetClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
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

    return smartAccount;
}
