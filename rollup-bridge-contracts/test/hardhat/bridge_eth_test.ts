import { expect } from "chai";
import "@nomicfoundation/hardhat-ethers";
import {
    ProcessedReceipt,
} from "@nilfoundation/niljs";
import "dotenv/config";
import type { Abi } from "abitype";
import { encodeFunctionData } from "viem";
import { generateL2RelayMessage } from "./generate-l2-relay-message";
import { bigIntReplacer } from "../../deploy/config/config-helper";
import { L2BridgeTestFixtureResult, setupL2BridgeTestFixture } from "./l2-bridge-test-fixture";

const l1EthBridgeAddress = '0x0001e0d8f4De4E838a66963f406Fa826cCaCA322';

describe("L2BridgeMessenger Contract", () => {
    it("Should accept the (ETHDeposit) message relayed by relayer", async () => {

        const l2BridgeTestFixture: L2BridgeTestFixtureResult = await setupL2BridgeTestFixture();

        // Relay DepositMessage to L2
        const depositorAddressValue = "0xc8d5559BA22d11B0845215a781ff4bF3CCa0EF89";
        const depositAmountValue = "1000000000000";
        const l2DepositRecipientValue = l2BridgeTestFixture.depositRecipientSmartAccount.address;
        const l2FeeRefundAddressValue = l2BridgeTestFixture.feeRefundSmartAccount.address;

        try {
            const messageSender = l1EthBridgeAddress;
            const messageTarget = l2BridgeTestFixture.l2ETHBridgeProxyAddress;
            const messageType = "1";
            const messageCreatedAt = Math.floor(Date.now() / 1000);
            const messageExpiryTime = Math.floor(Date.now() / 1000) + 10000;
            const ethDepositRelayMessage = generateL2RelayMessage(depositorAddressValue, depositAmountValue, l2DepositRecipientValue, l2FeeRefundAddressValue);
            const nilGasLimit = "1000000";
            const maxFeePerGas = "27500000";
            const maxPriorityFeePerGas = "1250000";
            const feeCredit = "27500000000000";
            const messageNonce = 0;

            // Dynamically load artifacts
            const L2BridgeMessengerJson = await import("../../artifacts/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json");

            if (!L2BridgeMessengerJson || !L2BridgeMessengerJson.default || !L2BridgeMessengerJson.default.abi || !L2BridgeMessengerJson.default.bytecode) {
                throw Error(`Invalid L2BridgeMessengerJson ABI`);
            }

            const relayMessage = encodeFunctionData({
                abi: L2BridgeMessengerJson.default.abi as Abi,
                functionName: "relayMessage",
                args: [messageSender, messageTarget, messageType, messageNonce, ethDepositRelayMessage, messageExpiryTime],
            });

            console.log(`generated message to relay on L2BridgeMessenger: ${relayMessage}`);

            const relayEthDepositMessageResponse = await l2BridgeTestFixture.ownerSmartAccount.sendTransaction({
                to: l2BridgeTestFixture.l2BridgeMessengerProxyAddress as `0x${string}`,
                data: relayMessage,
                feeCredit: BigInt(feeCredit),
                maxFeePerGas: BigInt(maxFeePerGas),
                maxPriorityFeePerGas: BigInt(maxPriorityFeePerGas)
            });

            console.log(`relayMessage transaction was done and awaiting for ProcessedReceipt`);

            const relayEthDepositMessageTxnReceipts: ProcessedReceipt[] = await relayEthDepositMessageResponse.wait();

            const relayedEthDepositMessageTxnReceipt: ProcessedReceipt = relayEthDepositMessageTxnReceipts[0] as ProcessedReceipt;

            const outputReceipts: ProcessedReceipt[] = relayedEthDepositMessageTxnReceipt.outputReceipts as ProcessedReceipt[];

            console.log(`outputReceipt is: ${JSON.stringify(outputReceipts, bigIntReplacer, 2)}`)

            const outputReceipt: ProcessedReceipt = outputReceipts[0] as ProcessedReceipt;

            console.log(`outputReceipt extracted as: ${JSON.stringify(outputReceipt, bigIntReplacer, 2)}`);

            // check the first element in the ProcessedReceipt and verify if it is successful
            if (!outputReceipt.success) {
                console.error(`Failed to relay message
            on the L2BridgeMessenger contract: ${l2BridgeTestFixture.l2BridgeMessengerProxyAddress}`);
            } else {
                console.log(`successfully relayed EthDepositMessage on to L2BridgeMessenger with transactionReceipt: ${JSON.stringify(relayEthDepositMessageTxnReceipts[0], bigIntReplacer, 2)}`);
            }
        } catch (err) {
            console.error(`Failed to relay ethBridge message on to L2BridgeMessenger: ${JSON.stringify(err)}`);
        }
    });
});
