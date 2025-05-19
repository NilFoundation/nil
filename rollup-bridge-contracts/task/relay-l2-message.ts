import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    FaucetClient,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    SmartAccountV1,
    convertEthToWei,
    Transaction,
    generateRandomPrivateKey,
    waitTillCompleted,
    getContract,
    ProcessedReceipt,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress } from "../scripts/utils/validate-config";
import { decodeFunctionResult, encodeFunctionData } from "viem";

// npx hardhat relay-l2-message --networkname local
task("relay-l2-message", "relay the l1-rbidge event data as message on to L2BridgeMessenger from Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2BridgeMessengerJson = await import("../artifacts/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json");
        if (!L2BridgeMessengerJson || !L2BridgeMessengerJson.default || !L2BridgeMessengerJson.default.abi || !L2BridgeMessengerJson.default.bytecode) {
            throw Error(`Invalid L2BridgeMessengerJson ABI`);
        }

        const networkName = taskArgs.networkname;
        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        // save the L2BridgeMessenger Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        const messageSender = "0x4735b6AEe529640b265D50d1409c4503e4Ef2c23";
        const messageTarget = "0x00012B0Cb7E43d2C7afBb4FD56d27Ea4Ee46Ad8a";
        const messageType = 1;
        const messageNonce = 10;
        const message = "0x3d6cec8c000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266000000000000000000000000000000000000000000000000000000e8d4a5100000000000000000000000000000017738fceb3c66d324b7845d60a5a456053be000000000000000000000000000012062f5b81613471751e2c4ff8a30eac06b2b";
        const messageExpiryTime = 1747681666;

        const relayMessage = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi as Abi,
            functionName: "relayMessage",
            args: [messageSender, messageTarget, messageType, messageNonce, message, messageExpiryTime],
        });

        const relayEthDepositMessageResponse = await deployerAccount.sendTransaction({
            to: l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy as `0x${string}`,
            data: relayMessage,
            feeCredit: convertEthToWei(0.001),
        });

        const relayEthDepositMessageTxnReceipts: ProcessedReceipt[] = await relayEthDepositMessageResponse.wait();

        const relayedEthDepositMessageTxnReceipt: ProcessedReceipt = relayEthDepositMessageTxnReceipts[0] as ProcessedReceipt;

        const outputReceipts: ProcessedReceipt[] = relayedEthDepositMessageTxnReceipt.outputReceipts as ProcessedReceipt[];

        const outputReceipt: ProcessedReceipt = outputReceipts[0] as ProcessedReceipt;

        console.log(`outputReceipt extracted as: ${JSON.stringify(outputReceipt, bigIntReplacer, 2)}`);

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!outputReceipt.success) {
            throw Error(`Failed to relay message
            on the L2BridgeMessenger contract: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`);
        }

        console.log(`successfully relayed EthDepositMessage on to L2BridgeMessenger with transactionReceipt: ${JSON.stringify(relayEthDepositMessageTxnReceipts[0], bigIntReplacer, 2)}`);
    });

export function bigIntReplacer(unusedKey: string, value: unknown): unknown {
    return typeof value === "bigint" ? value.toString() : value;
}