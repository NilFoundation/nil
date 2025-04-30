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
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";

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
    console.log("🟢 Loaded Smart Account:", smartAccount.address);

    return smartAccount;
}

export async function generateNilSmartAccount(networkName: string): Promise<SmartAccountV1> {
    const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;
    const client = new PublicClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });
    const faucetClient = new FaucetClient({
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
        console.log("🟢 Loaded Smart Account:", smartAccount.address);
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
        console.log("🆕 New Smart Account Generated:", smartAccount.address);
    }

    const topUpFaucet = await faucetClient.topUp({
        smartAccountAddress: smartAccount.address,
        amount: convertEthToWei(0.1),
        faucetAddress: process.env.NIL as `0x${string}`,
    });

    await waitTillCompleted(client, topUpFaucet);

    if ((await smartAccount.checkDeploymentStatus()) === false) {
        await smartAccount.selfDeploy(true);
    }

    console.log("✅ Smart Account Funded (100 ETH)");

    // update 
    const config: L2NetworkConfig = loadNilNetworkConfig(networkName);

    config.l2CommonConfig.owner = smartAccountAddress;
    config.l2CommonConfig.admin = smartAccountAddress;
    config.l2BridgeMessengerConfig.l2BridgeMessengerDeployerConfig.relayerAddress = smartAccountAddress;

    // Save the updated config
    saveNilNetworkConfig(networkName, config);

    return smartAccount;
}
