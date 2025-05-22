import type { Abi } from "abitype";
import { task } from "hardhat/config";
import { getContract } from "@nilfoundation/niljs";
import { loadL2DepositRecipientSmartAccount, loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadL1NetworkConfig, loadNilNetworkConfig } from "../deploy/config/config-helper";

// npx hardhat validate-l2-eth-bridging --networkname local
task("validate-l2-eth-bridging", "Validates the state changes of l2BridgeMessenger after ethBridge data is relayed to Nil")
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

        // save the L2BridgeMessenger Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        // verify if the bridges are really authorised
        const l2BridgeMessengerProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            address: l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy as `0x${string}`
        });

        const messageHash = l2NetworkConfig.l2TestConfig.messageSentEvent.messageHash;

        if (!messageHash) {
            throw Error(`DepositMessageHash is invalid in testJson which is to be asserted on L2BridgeMessenger`);
        }

        const retries: number = 10
        let isDepositMessageRelayed: boolean = false;

        for (let i = 0; i < 6; i++) {
            console.log(`Verifying relayedMessageHash: ${messageHash} inclusion in L2BridgeMessenger: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}... attempt ${i + 1}/${retries}`);
            try {
                isDepositMessageRelayed = await l2BridgeMessengerProxyInstance.read.isDepositMessageRelayed([messageHash]);
                if (isDepositMessageRelayed) {
                    break;
                }
            } catch (error) {
                console.error(`Error verifying relayedMessageHash: ${messageHash} inclusion in L2BridgeMessenger: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`, error);
            }
            if (i < retries - 1) {
                console.log(`Retrying verification... (${i + 1}/${retries})`);
                await sleepInMilliSeconds(1000 * Math.pow(2, i)); // Exponential backoff delay
            } else {
                throw new Error(
                    `Failed to verify relayedMessageHash: ${messageHash} inclusion in L2BridgeMessenger: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy} after ${retries} attempts`,
                );
            }
        }

        if (!isDepositMessageRelayed) {
            throw Error(`Failed to verify relayedMessageHash: ${messageHash} inclusion in L2BridgeMessenger: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`);
        }

        console.log(`Successfully verified that relayedMessageHash: ${messageHash} inclusion in L2BridgeMessenger: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`);

        // get depositRecipientBalance
        const depositRecipientSmartAccount = await loadL2DepositRecipientSmartAccount();

        if (!depositRecipientSmartAccount) {
            throw Error(`Invalid depositRecipientSmartAccount`);
        }

        const ethBalanceBefBridge = l2NetworkConfig.l2TestConfig.ethBalanceBefBridge;
        const ethBalanceAftBridgeFinality = await depositRecipientSmartAccount.getBalance();
        const depositAmount = loadL1NetworkConfig("geth").l1TestConfig.l1ETHDepositTestConfig.amount;

        // Convert depositAmount to BigInt for comparison
        const diffBalance = BigInt(ethBalanceAftBridgeFinality) - BigInt(ethBalanceBefBridge);

        if (diffBalance !== BigInt(depositAmount)) {
            throw new Error(`Balance mismatch: Expected ${BigInt(depositAmount)}, but got ${diffBalance}`);
        }

        if (!(ethBalanceAftBridgeFinality > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${depositRecipientSmartAccount.address}`);
        }

        console.log(`Deposit-recipient has ETH balance: ${ethBalanceAftBridgeFinality}`);
    });

function sleepInMilliSeconds(ms: number) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
