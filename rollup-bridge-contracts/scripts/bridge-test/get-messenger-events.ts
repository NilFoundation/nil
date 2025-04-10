import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';

export const messageSentEventABI = {
    anonymous: false,
    inputs: [
        { indexed: true, internalType: "address", name: "messageSender", type: "address" },
        { indexed: true, internalType: "address", name: "messageTarget", type: "address" },
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

export async function extractAndParseMessageSentEventLog(transactionHash: string,): Promise<MessageSentEvent | undefined> {
    const topic = "0xbfb3547e572ab179830e84cfa839c8af59c5d574a07bd2dec32b18780fd1db15";
    const transactionReceipt = await ethers.provider.getTransactionReceipt(transactionHash);

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

export function bigIntReplacer(unusedKey: string, value: unknown): unknown {
    return typeof value === "bigint" ? value.toString() : value;
}
