import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    convertEthToWei,
    getContract,
    ProcessedReceipt,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress, validateAddress } from "../scripts/utils/validate-config";
import { encodeFunctionData } from "viem";
import { fetchRelayer } from "./fetch-relayeraddress-from-relayer";

// npx hardhat grant-relayer-role --networkname local
task("grant-relayer-role", "Grant relayer role to the smart-account of relayer node")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs, hre) => {
        const networkName = taskArgs.networkname;

        // Dynamically load artifacts
        const L2BridgeMessengerJson = await import("../artifacts/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json");
        if (!L2BridgeMessengerJson || !L2BridgeMessengerJson.default || !L2BridgeMessengerJson.default.abi || !L2BridgeMessengerJson.default.bytecode) {
            throw Error(`Invalid L2BridgeMessengerJson ABI`);
        }

        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        validateAddress(l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy, "l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy");

        const relayerAddress = await fetchRelayer();
        l2NetworkConfig.l2CommonConfig.relayer = relayerAddress;

        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);

        const grantRelayerRoleTxnData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi as Abi,
            functionName: "grantRelayerRole",
            args: [getCheckSummedAddress(l2NetworkConfig.l2CommonConfig.relayer)],
        });

        const grantRelayerRoleResponse = await deployerAccount.sendTransaction({
            to: l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy as `0x${string}`,
            data: grantRelayerRoleTxnData,
            feeCredit: convertEthToWei(0.001),
        });

        const grantRelayerRoleResponseTxnReceipt: ProcessedReceipt[] = await grantRelayerRoleResponse.wait();

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!grantRelayerRoleResponse[0].success) {
            throw Error(`Failed to grant relayerRole for: ${l2NetworkConfig.l2CommonConfig.relayer} 
            on the L2EnshrinedTokenBridge contract: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`);
        }

        // verify if the relayer role is granted
        const l2BridgeMessengerProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            address: l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy as `0x${string}`
        });

        const hasRelayerRole = await l2BridgeMessengerProxyInstance.read.hasRelayerRole([l2NetworkConfig.l2CommonConfig.relayer]);
        if (!hasRelayerRole) {
            throw Error(`RELAYER role is not granted for ${l2NetworkConfig.l2CommonConfig.relayer} on L2BridgeMessenger`);
        }
    });
