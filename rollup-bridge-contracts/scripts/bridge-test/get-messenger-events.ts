import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';

export const messageSentEventABI = {
    anonymous: false,
    inputs: [
        { indexed: true, internalType: "address", name: "messageSender", type: "address" },
        { indexed: true, internalType: "address", name: "messageTarget", type: "address" },
        { indexed: false, internalType: "uint256", name: "messageValue", type: "uint256" },
        { indexed: true, internalType: "uint256", name: "messageNonce", type: "uint256" },
        { indexed: false, internalType: "bytes", name: "message", type: "bytes" },
        { indexed: false, internalType: "bytes32", name: "messageHash", type: "bytes32" },
        { indexed: false, internalType: "enum NilConstants.MessageType", name: "messageType", type: "uint8" },
        { indexed: false, internalType: "uint256", name: "messageCreatedAt", type: "uint256" },
        { indexed: false, internalType: "uint256", name: "messageExpiryTime", type: "uint256" },
        { indexed: false, internalType: "address", name: "l2FeeRefundAddress", type: "address" },
        {
            components: [
                { internalType: "uint256", name: "nilGasLimit", type: "uint256" },
                { internalType: "uint256", name: "maxFeePerGas", type: "uint256" },
                { internalType: "uint256", name: "maxPriorityFeePerGas", type: "uint256" },
                { internalType: "uint256", name: "feeCredit", type: "uint256" },
            ],
            indexed: false,
            internalType: "struct INilGasPriceOracle.FeeCreditData",
            name: "feeCreditData",
            type: "tuple",
        },
    ],
    name: "MessageSent",
    type: "event",
};

export type MessageSentEvent = {
    messageSender: string;
    messageTarget: string;
    messageValue: string;
    messageNonce: string;
    message: string;
    messageHash: string;
    messageType: number;
    messageCreatedAt: string;
    messageExpiryTime: string;
    l2FeeRefundAddress: string;
    feeCreditData: {
        nilGasLimit: string;
        maxFeePerGas: string;
        maxPriorityFeePerGas: string;
        feeCredit: string;
    };
};

export const depositERC20EventABI = {
    anonymous: false,
    inputs: [
        {
            indexed: true,
            internalType: "address",
            name: "l1Token",
            type: "address",
        },
        {
            indexed: true,
            internalType: "address",
            name: "l2Token",
            type: "address",
        },
        {
            indexed: true,
            internalType: "address",
            name: "depositor",
            type: "address",
        },
        {
            indexed: false,
            internalType: "address",
            name: "l2Recipient",
            type: "address",
        },
        {
            indexed: false,
            internalType: "uint256",
            name: "amount",
            type: "uint256",
        },
        {
            indexed: false,
            internalType: "bytes",
            name: "data",
            type: "bytes",
        },
    ],
    name: "DepositERC20",
    type: "event",
};

// 0x31cd3b976e4d654022bf95c68a2ce53f1d5d94afabe0454d2832208eeb40af25

export type DepositERC20Event = {
    l1Token: string;
    l2Token: string;
    depositor: string;
    l2Recipient: string;
    amount: string; // Representing BigNumber as a string
    data: string;
};

export async function extractAndParseDepositERC20Event(transactionHash: string): Promise<DepositERC20Event | null> {
    const topic = "0x31cd3b976e4d654022bf95c68a2ce53f1d5d94afabe0454d2832208eeb40af25";
    const transactionReceipt = await ethers.provider.getTransactionReceipt(transactionHash);

    const filteredLogs = transactionReceipt.logs.filter((log: any) =>
        log.topics.includes(topic)
    );
    if (filteredLogs.length === 0) {
        console.log(`No logs found with topic: ${topic}`);
        return null;
    }

    const iface = new ethers.Interface([depositERC20EventABI]);
    const parsedLog = iface.parseLog(filteredLogs[0]);

    const depositERC20EventDetails: DepositERC20Event = {
        l1Token: parsedLog.args.l1Token,
        l2Token: parsedLog.args.l2Token,
        depositor: parsedLog.args.depositor,
        l2Recipient: parsedLog.args.l2Recipient,
        amount: parsedLog.args.amount.toString(), // Convert BigNumber to string
        data: parsedLog.args.data,
    };

    return depositERC20EventDetails;
}

export async function extractAndParseMessageSentEventLog(transactionHash: string): Promise<MessageSentEvent | undefined> {
    const topic = "0x29820d80f4b3b15e5871bf7c904640ab18dff6fe6b7839e60303fe6f8539ec7c";
    const transactionReceipt = await ethers.provider.getTransactionReceipt(transactionHash);

    console.log(`Transaction Receipt: ${JSON.stringify(transactionReceipt, null, 2)}`);

    // Filter logs by the specific topic
    const filteredLogs = transactionReceipt.logs.filter((log: any) =>
        log.topics.includes(topic)
    );

    if (filteredLogs.length === 0) {
        console.log(`No logs found with topic: ${topic}`);
        return;
    }

    //console.log(`Filtered Logs with topic ${topic}:`);
    filteredLogs.forEach((log: any, index: number) => {
        //console.log(`Log ${index + 1}:`, log);
    });

    const iface = new ethers.Interface([messageSentEventABI]);
    const parsedLog = iface.parseLog(filteredLogs[0]);

    const eventDetails: MessageSentEvent = {
        messageSender: parsedLog.args.messageSender,
        messageTarget: parsedLog.args.messageTarget,
        messageValue: parsedLog.args.messageValue.toString(),
        messageNonce: parsedLog.args.messageNonce.toString(),
        message: parsedLog.args.message,
        messageHash: parsedLog.args.messageHash,
        messageType: parsedLog.args.messageType,
        messageCreatedAt: parsedLog.args.messageCreatedAt.toString(),
        messageExpiryTime: parsedLog.args.messageExpiryTime.toString(),
        l2FeeRefundAddress: parsedLog.args.l2FeeRefundAddress,
        feeCreditData: {
            nilGasLimit: parsedLog.args.feeCreditData.nilGasLimit.toString(),
            maxFeePerGas: parsedLog.args.feeCreditData.maxFeePerGas.toString(),
            maxPriorityFeePerGas: parsedLog.args.feeCreditData.maxPriorityFeePerGas.toString(),
            feeCredit: parsedLog.args.feeCreditData.feeCredit.toString(),
        },
    };

    return eventDetails;
}

// npx hardhat run scripts/bridge-test/get-messenger-events.ts --network geth
async function main() {
    const transactionHash = "0xf83722ed8464f5a0e5492a89de0a4b869a82288d694ff17443c9749059797754";
    const messageSentEventLogData = await extractAndParseMessageSentEventLog(transactionHash);

    if (messageSentEventLogData) {
        const messageSentEvent: MessageSentEvent = messageSentEventLogData;
        console.log(`MessageSentEvent is: ${JSON.stringify(messageSentEvent, bigIntReplacer, 2)}`);
    }

    const depositERC20EventLogData = await extractAndParseDepositERC20Event(transactionHash);
    console.log(`depositERC20EventLogData is: ${JSON.stringify(depositERC20EventLogData, bigIntReplacer, 2)}`);
}

export function bigIntReplacer(unusedKey: string, value: unknown): unknown {
    return typeof value === "bigint" ? value.toString() : value;
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
