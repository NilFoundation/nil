import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    convertEthToWei,
    getContract,
    ProcessedReceipt,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L1NetworkConfig, L2NetworkConfig, loadL1NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress, validateAddress } from "../scripts/utils/validate-config";
import { encodeFunctionData } from "viem";

// npx hardhat set-eth-bridge-dependencies --networkname local
task("set-eth-bridge-dependencies", "Set dependencies of L2ETHBridge contract")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .addParam("l1networkname", "The l1 network to use") // Mandatory parameter
    .setAction(async (taskArgs, hre) => {
        const networkName = taskArgs.networkname;
        const l1NetworkName = taskArgs.l1networkname;

        // Dynamically load artifacts
        const L2ETHBridgeJson = await import("../artifacts/contracts/bridge/l2/L2ETHBridge.sol/L2ETHBridge.json");
        if (!L2ETHBridgeJson || !L2ETHBridgeJson.default || !L2ETHBridgeJson.default.abi || !L2ETHBridgeJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridgeJson ABI`);
        }

        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        // save the L2BridgeMessenger Address in the json config for l2
        const l1NetworkConfig: L1NetworkConfig = loadL1NetworkConfig(l1NetworkName);

        validateAddress(l1NetworkConfig.l1ETHBridge.l1ETHBridgeProxy, "l1NetworkConfig.l1ETHBridge.l1ETHBridgeProxy");

        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        validateAddress(l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy, "l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy");

        const setCounterPartyBridgeData = encodeFunctionData({
            abi: L2ETHBridgeJson.default.abi as Abi,
            functionName: "setCounterpartyBridge",
            args: [getCheckSummedAddress(l1NetworkConfig.l1ETHBridge.l1ETHBridgeProxy)],
        });

        const setCounterPartyBridgeResponse = await deployerAccount.sendTransaction({
            to: l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy as `0x${string}`,
            data: setCounterPartyBridgeData,
            feeCredit: convertEthToWei(0.001),
        });

        const setCounterPartyBridgeResponseTxnReceipt: ProcessedReceipt[] = await setCounterPartyBridgeResponse.wait();

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!setCounterPartyBridgeResponseTxnReceipt[0].success) {
            throw Error(`Failed to set CounterpartyBridge: ${l1NetworkConfig.l1ETHBridge.l1ETHBridgeProxy} 
            on the L2ETHBridge contract: ${l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy}`);
        }

        // verify if the CounterpartyBridge is set
        const l2ETHBridgeProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2ETHBridgeJson.default.abi as Abi,
            address: l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy as `0x${string}`
        });

        const CounterpartyBridgeFromL2ETHBridgeContract = await l2ETHBridgeProxyInstance.read.CounterpartyBridge([]);
        if (!CounterpartyBridgeFromL2ETHBridgeContract || CounterpartyBridgeFromL2ETHBridgeContract != getCheckSummedAddress(l1NetworkConfig.l1ETHBridge.l1ETHBridgeProxy)) {
            throw Error(`Invalid counterpartyBridge: ${CounterpartyBridgeFromL2ETHBridgeContract} was set in L2ETHBridge. expected counterpartyBridge is: ${getCheckSummedAddress(l1NetworkConfig.l1ETHBridge.l1ETHBridgeProxy)}`);
        }
    });
