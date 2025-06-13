import { convertEthToWei, SmartAccountV1, waitTillCompleted } from "@nilfoundation/niljs";
import { readFileSync } from "fs";
import { loadNilNetworkConfig } from "../../deploy/config/config-helper";
import { Abi, encodeFunctionData } from "viem";
import { bigIntReplacer } from "./get-messenger-events";
import path from "path";

const L2ETHBridgeABI = JSON.parse(readFileSync(
    path.join(
        __dirname,
        `../../artifacts/contracts/bridge/l2/L2ETHBridge.sol/L2ETHBridge.json`,
    ), 'utf8')
).abi;

export async function withdrawETH(
    network: string,
    src: SmartAccountV1,
    dst: string,
    amount: BigInt,
) {
    const nilNetworkConfig = loadNilNetworkConfig(network);
    const L2ETHBridgeProxyAddr = nilNetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy as `0x${string}`;

    console.log(`Using L2ETHBridgeProxyAddr: ${L2ETHBridgeProxyAddr}`);

    const data = encodeFunctionData({
        abi: L2ETHBridgeABI as Abi,
        functionName: "withdrawETH",
        args: [dst, amount],
    });

    const tx = await src.sendTransaction({
        to: L2ETHBridgeProxyAddr,
        data: data,
        feeCredit: convertEthToWei(0.001),
    });

    console.log(`Withdraw ETH transaction sent: ${tx.hash}`);

    const receipt = await tx.wait();
    if (receipt.some((r) => !r.success)) {
        console.log(
          `Transaction ${tx.hash} failed. Receipts: ${JSON.stringify(receipt, bigIntReplacer)}`,
        );
    } else {
        console.log(`Withdraw ETH transaction successful: ${tx.hash}`);
        console.log(`Transaction receipts: ${JSON.stringify(receipt, bigIntReplacer)}`);
    }
}
