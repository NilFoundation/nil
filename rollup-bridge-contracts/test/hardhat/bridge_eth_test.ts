import { expect } from "chai";
import "@nomicfoundation/hardhat-ethers";
import {
    convertEthToWei,
    FaucetClient,
    HttpTransport,
    ProcessedReceipt,
    PublicClient,
    SmartAccountV1,
    waitTillCompleted,
    getContract
} from "@nilfoundation/niljs";
import "dotenv/config";
import type { Abi } from "abitype";
import { getCheckSummedAddress } from "../../scripts/utils/validate-config";
import { decodeFunctionResult, encodeFunctionData } from "viem";
import { generateNilSmartAccount, loadNilSmartAccount, prepareNilSmartAccountsForUnitTest } from "../../task/nil-smart-account";
import { generateL2RelayMessage } from "./generate-l2-relay-message";
import { bigIntReplacer } from "../../deploy/config/config-helper";

const l1EthBridgeAddress = '0x0001e0d8f4De4E838a66963f406Fa826cCaCA322';

describe("L2BridgeMessenger Contract", () => {
    it("Should accept the (ETHDeposit) message relayed by relayer", async () => {

        // setup
        let ownerSmartAccount: SmartAccountV1 | null = null;
        let depositRecipientSmartAccount: SmartAccountV1 | null = null;
        let feeRefundSmartAccount: SmartAccountV1 | null = null;

        try {
            const result = await prepareNilSmartAccountsForUnitTest();
            ownerSmartAccount = result.ownerSmartAccount;
            depositRecipientSmartAccount = result.depositRecipientSmartAccount;
            feeRefundSmartAccount = result.feeRefundSmartAccount;
        } catch (err) {
            console.error(`Failed to load NilSmartAccount - 1st catch: ${JSON.stringify(err)}`);
            return;
        }

        if (!ownerSmartAccount || !depositRecipientSmartAccount || !feeRefundSmartAccount) {
            console.error(`Failed to load all required SmartAccounts`);
            // Optionally: expect.fail("Failed to load all required SmartAccounts");
        }

        console.log(`loaded smart-account successfully`);

        // // ##### Fund Deployer Wallet #####
        const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;

        try {
            const client = new PublicClient({
                transport: new HttpTransport({ endpoint: rpcEndpoint }),
            });
            const faucetClient = new FaucetClient({
                transport: new HttpTransport({ endpoint: rpcEndpoint }),
            });

            const topUpFaucetTxnHash = await faucetClient.topUp({
                smartAccountAddress: ownerSmartAccount.address,
                amount: convertEthToWei(100),
                faucetAddress: process.env.NIL as `0x${string}`,
            });

            await waitTillCompleted(client, topUpFaucetTxnHash);

            const balance = await ownerSmartAccount.getBalance();

            if (!(balance > BigInt(0))) {
                throw Error(`Insufficient or Zero balance for smart-account: ${ownerSmartAccount.address}`);
            }
        } catch (err) {
            console.error(`Failed to topup nil-smartAccount: ${ownerSmartAccount.address}`);
        }

        // ##### NilMessageTree Deployment ##### 

        // Dynamically load artifacts
        const NilMessageTreeJson = await import("../../artifacts/contracts/common/NilMessageTree.sol/NilMessageTree.json");

        if (!NilMessageTreeJson || !NilMessageTreeJson.default || !NilMessageTreeJson.default.abi || !NilMessageTreeJson.default.bytecode) {
            throw Error(`Invalid NilMessageTree ABI`);
        }

        const { tx: nilMessageTreeDeployTxn, address: nilMessageTreeAddress } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: NilMessageTreeJson.default.bytecode as `0x${string}`,
            abi: NilMessageTreeJson.default.abi as Abi,
            args: [ownerSmartAccount.address],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(nilMessageTreeDeployTxn, ownerSmartAccount.client, "NilMessageTree");

        if (!nilMessageTreeDeployTxn.hash) {
            throw Error(`Invalid transaction output from deployContract call for NilMessageTree Contract`);
        }

        if (!nilMessageTreeAddress) {
            throw Error(`Invalid address output from deployContract call for NilMessageTree Contract`);
        }

        // ##### L2ETHBridgeVault Deployment #####

        // Dynamically load artifacts
        const L2ETHBridgeVaultJson = await import("../../artifacts/contracts/bridge/l2/L2ETHBridgeVault.sol/L2ETHBridgeVault.json");
        const TransparentUpgradeableProxy = await import("../../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");

        if (!L2ETHBridgeVaultJson || !L2ETHBridgeVaultJson.default || !L2ETHBridgeVaultJson.default.abi || !L2ETHBridgeVaultJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridgeVault ABI`);
        }

        const { tx: l2EthBridgeVaultImplementationDeploymentTx, address: l2EthBridgeVaultImplementationAddress } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeVaultJson.default.bytecode as `0x${string} `,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001)
        });

        await verifyDeploymentCompletion(l2EthBridgeVaultImplementationDeploymentTx, ownerSmartAccount.client, "L2ETHBridgeVault");

        if (!l2EthBridgeVaultImplementationDeploymentTx || !l2EthBridgeVaultImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridgeVault Contract`);
        }

        if (!l2EthBridgeVaultImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridgeVault Contract`);
        }

        const l2EthBridgeVaultInitData = encodeFunctionData({
            abi: L2ETHBridgeVaultJson.default.abi,
            functionName: "initialize",
            args: [ownerSmartAccount.address, ownerSmartAccount.address],
        });

        const { tx: l2EthBridgeVaultProxyDeploymentTx, address: l2EthBridgeVaultProxy } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EthBridgeVaultImplementationAddress, ownerSmartAccount.address, l2EthBridgeVaultInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EthBridgeVaultProxyDeploymentTx, ownerSmartAccount.client, "L2ETHBridgeVaultProxy");

        const faucetClient = new FaucetClient({
            transport: new HttpTransport({ endpoint: rpcEndpoint }),
        });

        const topUpFaucet = await faucetClient.topUp({
            smartAccountAddress: l2EthBridgeVaultProxy as `0x${string} `,
            amount: convertEthToWei(200),
            faucetAddress: process.env.NIL as `0x${string} `,
        });

        const fundL2ETHBridgeVaultTxnReceipts: ProcessedReceipt[] = await waitTillCompleted(ownerSmartAccount.client, topUpFaucet);

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!fundL2ETHBridgeVaultTxnReceipts[0].success) {
            throw Error(`Failed to fund L2ETHBridgeVault: ${l2EthBridgeVaultProxy} `);
        }

        const balanceAfterFunding = await ownerSmartAccount.client.getBalance(l2EthBridgeVaultProxy as `0x${string} `);

        const l2EthBridgeVaultProxyAddress = getCheckSummedAddress(l2EthBridgeVaultProxy);

        // ##### L2BridgeMessenger Deployment ##### 

        // Dynamically load artifacts
        const L2BridgeMessengerJson = await import("../../artifacts/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json");

        if (!L2BridgeMessengerJson || !L2BridgeMessengerJson.default || !L2BridgeMessengerJson.default.abi || !L2BridgeMessengerJson.default.bytecode) {
            throw Error(`Invalid L2BridgeMessengerJson ABI`);
        }

        const { tx: nilMessengerImplementationDeploymentTx, address: nilMessengerImplementationAddress } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: L2BridgeMessengerJson.default.bytecode as `0x${string} `,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(nilMessengerImplementationDeploymentTx, ownerSmartAccount.client, "L2BridgeMessenger");


        if (!nilMessengerImplementationDeploymentTx || !nilMessengerImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2BridgeMessenger Contract`);
        }

        if (!nilMessengerImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2BridgeMessenger Contract`);
        }

        const l2BridgeMessengerImplementationAddress = getCheckSummedAddress(nilMessengerImplementationAddress);

        const l2BridgeMessengerInitData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi,
            functionName: "initialize",
            args: [ownerSmartAccount.address,
            ownerSmartAccount.address,
            ownerSmartAccount.address,
                nilMessageTreeAddress,
                1000000],
        });

        const { tx: l2BridgeMessengerProxyDeploymentTx, address: l2BridgeMessengerProxy } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2BridgeMessengerImplementationAddress, ownerSmartAccount.address, l2BridgeMessengerInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2BridgeMessengerProxyDeploymentTx, ownerSmartAccount.client, "L2BridgeMessengerProxy");

        const l2BridgeMessengerProxyAddress = getCheckSummedAddress(l2BridgeMessengerProxy);

        let l2BridgeMessengerProxyInst;

        try {
            // verify if the bridges are really authorised
            l2BridgeMessengerProxyInst = getContract({
                client: ownerSmartAccount.client,
                abi: L2BridgeMessengerJson.default.abi as Abi,
                address: l2BridgeMessengerProxyAddress as `0x${string} `
            });

        } catch (err) {
            console.error(`Error caught while loading an instance of L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `);
        }

        // Dynamically load artifacts
        const L2ETHBridgeJson = await import("../../artifacts/contracts/bridge/l2/L2ETHBridge.sol/L2ETHBridge.json");

        if (!L2ETHBridgeJson || !L2ETHBridgeJson.default || !L2ETHBridgeJson.default.abi || !L2ETHBridgeJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridge ABI`);
        }

        // #####  l2ETHBridge Deployment ##### 

        const { tx: l2EthBridgeImplementationDeploymentTx, address: l2EthBridgeImplementation } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeJson.default.bytecode as `0x${string} `,
            abi: L2ETHBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EthBridgeImplementationDeploymentTx, ownerSmartAccount.client, "L2ETHBridge");

        if (!l2EthBridgeImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridge Contract`);
        }

        if (!l2EthBridgeImplementation) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridge Contract`);
        }

        const l2ETHBridgeImplementationAddress = getCheckSummedAddress(l2EthBridgeImplementation);

        const l2EthBridgeInitData = encodeFunctionData({
            abi: L2ETHBridgeJson.default.abi,
            functionName: "initialize",
            args: [ownerSmartAccount.address,
            ownerSmartAccount.address,
                l2BridgeMessengerProxyAddress,
                l2EthBridgeVaultProxyAddress],
        });
        const { tx: l2EthBridgeProxyDeploymentTx, address: l2EthBridgeProxy } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2ETHBridgeImplementationAddress, ownerSmartAccount.address, l2EthBridgeInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EthBridgeProxyDeploymentTx, ownerSmartAccount.client, "L2ETHBridgeProxy");

        const l2ETHBridgeProxyAddress = getCheckSummedAddress(l2EthBridgeProxy);

        // Dynamically load artifacts
        const L2EnshrinedTokenBridgeJson = await import("../../artifacts/contracts/bridge/l2/L2EnshrinedTokenBridge.sol/L2EnshrinedTokenBridge.json");

        if (!L2EnshrinedTokenBridgeJson || !L2EnshrinedTokenBridgeJson.default || !L2EnshrinedTokenBridgeJson.default.abi || !L2EnshrinedTokenBridgeJson.default.bytecode) {
            throw Error(`Invalid L2EnshrinedTokenBridge ABI`);
        }

        const { tx: l2EnshrinedTokenBridgeImplDepTx, address: l2EnshrinedTokenBridgeImpl } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: L2EnshrinedTokenBridgeJson.default.bytecode as `0x${string} `,
            abi: L2EnshrinedTokenBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EnshrinedTokenBridgeImplDepTx, ownerSmartAccount.client, "L2EnshrinedTokenBridge");

        if (!l2EnshrinedTokenBridgeImplDepTx || !l2EnshrinedTokenBridgeImplDepTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2EnshrinedTokenBridge Contract`);
        }

        if (!l2EnshrinedTokenBridgeImpl) {
            throw Error(`Invalid address output from deployContract call for L2EnshrinedTokenBridge Contract`);
        }

        const l2EnshrinedTokenBridgeImplementationAddress = getCheckSummedAddress(l2EnshrinedTokenBridgeImpl);

        const l2EnshrinedTokenBridgeInitData = encodeFunctionData({
            abi: L2EnshrinedTokenBridgeJson.default.abi,
            functionName: "initialize",
            args: [ownerSmartAccount.address,
            ownerSmartAccount.address,
                l2BridgeMessengerProxyAddress],
        });

        const { tx: l2EnshrinedTokenBridgeProxyDeploymentTx, address: l2EnshrinedTokenBridgeProxy } = await ownerSmartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EnshrinedTokenBridgeImplementationAddress, ownerSmartAccount.address, l2EnshrinedTokenBridgeInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EnshrinedTokenBridgeProxyDeploymentTx, ownerSmartAccount.client, "L2EnshrinedTokenBridgeProxy");


        const l2EnshrinedTokenBridgeProxyAddress = getCheckSummedAddress(l2EnshrinedTokenBridgeProxy);

        const authoriseBridgesData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi as Abi,
            functionName: "authoriseBridges",
            args: [[l2ETHBridgeProxyAddress,
                l2EnshrinedTokenBridgeProxyAddress]
            ],
        });

        let authoriseL2BridgesTxnReceipts: ProcessedReceipt[];

        try {
            const authoriseL2BridgesResponse = await ownerSmartAccount.sendTransaction({
                to: l2BridgeMessengerProxyAddress as `0x${string} `,
                data: authoriseBridgesData,
                feeCredit: convertEthToWei(0.001),
            });

            authoriseL2BridgesTxnReceipts = await authoriseL2BridgesResponse.wait();
        } catch (err) {
            console.error(`Error caught when the bridges are being authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `, err);
        }

        const authoriseL2Bridges_outputProcessedReceipt = authoriseL2BridgesTxnReceipts?.[0]?.outputReceipts?.[0];
        if (
            !authoriseL2Bridges_outputProcessedReceipt?.success ||
            authoriseL2Bridges_outputProcessedReceipt.status === '' ||
            (typeof authoriseL2Bridges_outputProcessedReceipt.status === 'string' && authoriseL2Bridges_outputProcessedReceipt.status.toLowerCase().includes('reverted'))
        ) {
            console.error(`❌ Failed to authorise Bridges: ${[l2ETHBridgeProxyAddress,
                l2EnshrinedTokenBridgeProxyAddress]} 
                                on the L2BridgeMessenger contract: ${l2BridgeMessengerProxyAddress}`);
        } else {
            console.log(`✅ Successfully authorised Bridges: ${[l2ETHBridgeProxyAddress,
                l2EnshrinedTokenBridgeProxyAddress]} 
                                on the L2BridgeMessenger contract: ${l2BridgeMessengerProxyAddress}`);
        }

        let l2BridgeMessengerProxyInstance;

        try {
            // verify if the bridges are really authorised
            l2BridgeMessengerProxyInstance = getContract({
                client: ownerSmartAccount.client,
                abi: L2BridgeMessengerJson.default.abi as Abi,
                address: l2BridgeMessengerProxyAddress as `0x${string}`
            });

            const isL2EnshrinedTokenBridgeAuthorised = await l2BridgeMessengerProxyInstance.read.isAuthorisedBridge([l2EnshrinedTokenBridgeProxyAddress]);
            if (!isL2EnshrinedTokenBridgeAuthorised) {
                console.error(`❌ L2EnshrinedTokenBridge: ${l2EnshrinedTokenBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
                //expect.fail(`L2EnshrinedTokenBridge: ${l2EnshrinedTokenBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
            }

            const isL2ETHBridgeAuthorised = await l2BridgeMessengerProxyInstance.read.isAuthorisedBridge([l2ETHBridgeProxyAddress]);
            if (!isL2ETHBridgeAuthorised) {
                console.error(`❌ L2ETHBridge: ${l2ETHBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
                expect.fail(`L2ETHBridge: ${l2ETHBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
            }

        } catch (err) {
            console.error(`❌ Error caught while getting an instance of L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `);
            expect.fail(`❌ Error caught while getting an instance of L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `);
        }


        const setL2ETHBridgeData = encodeFunctionData({
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            functionName: "setL2ETHBridge",
            args: [l2ETHBridgeProxyAddress],
        });

        const setL2ETHBridgeResponse = await ownerSmartAccount.sendTransaction({
            to: l2EthBridgeVaultProxyAddress as `0x${string}`,
            data: setL2ETHBridgeData,
            feeCredit: convertEthToWei(0.001),
        });

        const setL2ETHBridgeResponseTxnReceipt: ProcessedReceipt[] = await setL2ETHBridgeResponse.wait();


        const setL2ETHBridge_outputProcessedReceipt = setL2ETHBridgeResponseTxnReceipt?.[0]?.outputReceipts?.[0];
        if (
            !setL2ETHBridge_outputProcessedReceipt?.success ||
            setL2ETHBridge_outputProcessedReceipt.status === '' ||
            (typeof setL2ETHBridge_outputProcessedReceipt.status === 'string' && setL2ETHBridge_outputProcessedReceipt.status.toLowerCase().includes('reverted'))
        ) {
            console.error(`❌ Failed to wire L2ETHBridge: ${l2ETHBridgeProxyAddress} 
                               as dependency in the ETHBridgeVault contract: ${l2EthBridgeVaultProxyAddress}`);
        } else {
            console.log(`✅ Successfully wired L2ETHBridge: ${l2ETHBridgeProxyAddress} 
                                as dependency in the ETHBridgeVault contract: ${l2EthBridgeVaultProxyAddress}`);
        }

        // verify if the L2ETHBridge is set
        const l2ETHBridgeVaultProxyInstance = getContract({
            client: ownerSmartAccount.client,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            address: l2EthBridgeVaultProxyAddress as `0x${string}`
        });

        const l2ETHBridgeFromVaultContract = await l2ETHBridgeVaultProxyInstance.read.l2ETHBridge([]);
        if (!l2ETHBridgeFromVaultContract || l2ETHBridgeFromVaultContract != l2ETHBridgeProxyAddress) {
            throw Error(`Invalid L2ETHBridge: ${l2ETHBridgeFromVaultContract} was set in L2ETHBridgeVault. expected L2ETHBridge from Vault: ${l2ETHBridgeProxyAddress}`);
        }

        // TODO replace this with dummy contract deployed address
        const l1ETHBridgeProxyDummyAddress = l2ETHBridgeProxyAddress;

        const setCounterPartyBridgeData = encodeFunctionData({
            abi: L2ETHBridgeJson.default.abi as Abi,
            functionName: "setCounterpartyBridge",
            args: [getCheckSummedAddress(l1ETHBridgeProxyDummyAddress)],
        });

        const setCounterPartyBridgeResponse = await ownerSmartAccount.sendTransaction({
            to: l2ETHBridgeProxyAddress as `0x${string}`,
            data: setCounterPartyBridgeData,
            feeCredit: convertEthToWei(0.001),
        });

        const setCounterPartyETHBridge_Receipt: ProcessedReceipt[] = await setCounterPartyBridgeResponse.wait();

        const setCounterPartyETHBridge_outputProcessedReceipt = setCounterPartyETHBridge_Receipt?.[0]?.outputReceipts?.[0];
        if (
            !setCounterPartyETHBridge_outputProcessedReceipt?.success ||
            setCounterPartyETHBridge_outputProcessedReceipt.status === '' ||
            (typeof setCounterPartyETHBridge_outputProcessedReceipt.status === 'string' && setCounterPartyETHBridge_outputProcessedReceipt.status.toLowerCase().includes('reverted'))
        ) {
            console.error(`❌ Failed to set counterparty ETHBridge: ${l2ETHBridgeProxyAddress} 
                               as dependency in the L2ETHBridge contract: ${l2ETHBridgeProxyAddress}`);
        } else {
            console.log(`✅ Successfully set counterparty ETHBridge: ${l2ETHBridgeProxyAddress} 
                                as dependency in the L2ETHBridge contract: ${l2ETHBridgeProxyAddress}`);
        }

        // verify if the CounterpartyBridge is set
        const l2ETHBridgeProxyInstance = getContract({
            client: ownerSmartAccount.client,
            abi: L2ETHBridgeJson.default.abi as Abi,
            address: l2ETHBridgeProxyAddress as `0x${string}`
        });

        const counterpartyBridgeFromL2ETHBridgeContract = await l2ETHBridgeProxyInstance.read.counterpartyBridge([]);
        if (!counterpartyBridgeFromL2ETHBridgeContract || counterpartyBridgeFromL2ETHBridgeContract != getCheckSummedAddress(l1ETHBridgeProxyDummyAddress)) {
            throw Error(`Invalid counterpartyBridge: ${counterpartyBridgeFromL2ETHBridgeContract} was set in L2ETHBridge. expected counterpartyBridge is: ${getCheckSummedAddress(l1ETHBridgeProxyDummyAddress)}`);
        }


        const grantRelayerRoleTxnData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi as Abi,
            functionName: "grantRelayerRole",
            args: [getCheckSummedAddress(ownerSmartAccount.address)],
        });

        const grantRelayerRoleResponse = await ownerSmartAccount.sendTransaction({
            to: l2BridgeMessengerProxyAddress as `0x${string}`,
            data: grantRelayerRoleTxnData,
            feeCredit: convertEthToWei(0.001),
        });

        const grantRelayerRoleResponseTxnReceipt: ProcessedReceipt[] = await grantRelayerRoleResponse.wait();

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!grantRelayerRoleResponseTxnReceipt[0].success) {
            throw Error(`Failed to grant relayerRole for: ${ownerSmartAccount.address} 
            on the L2EnshrinedTokenBridge contract: ${l2BridgeMessengerProxyAddress}`);
        }

        const hasRelayerRole = await l2BridgeMessengerProxyInstance.read.hasRelayerRole([ownerSmartAccount.address]);
        if (!hasRelayerRole) {
            throw Error(`RELAYER role is not granted for ${ownerSmartAccount.address} on L2BridgeMessenger`);
        }

        console.log(`successfully granted RELAYER role for ${ownerSmartAccount.address} on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);

        const depositorAddressValue = "0xc8d5559BA22d11B0845215a781ff4bF3CCa0EF89";
        const depositAmountValue = "1000000000000";
        const l2DepositRecipientValue = depositRecipientSmartAccount.address;
        const l2FeeRefundAddressValue = feeRefundSmartAccount.address;

        try {
            const messageSender = l1ETHBridgeProxyDummyAddress;
            const messageTarget = l2ETHBridgeProxyAddress;
            const messageType = "1";
            const messageCreatedAt = Math.floor(Date.now() / 1000);
            const messageExpiryTime = Math.floor(Date.now() / 1000) + 10000;
            const ethDepositRelayMessage = generateL2RelayMessage(depositorAddressValue, depositAmountValue, l2DepositRecipientValue, l2FeeRefundAddressValue);
            const nilGasLimit = "1000000";
            const maxFeePerGas = "27500000";
            const maxPriorityFeePerGas = "1250000";
            const feeCredit = "27500000000000";
            const messageNonce = 0;

            const relayMessage = encodeFunctionData({
                abi: L2BridgeMessengerJson.default.abi as Abi,
                functionName: "relayMessage",
                args: [messageSender, messageTarget, messageType, messageNonce, ethDepositRelayMessage, messageExpiryTime],
            });

            console.log(`generated message to relay on L2BridgeMessenger: ${relayMessage}`);

            const relayEthDepositMessageResponse = await ownerSmartAccount.sendTransaction({
                to: l2BridgeMessengerProxyAddress as `0x${string}`,
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
            on the L2BridgeMessenger contract: ${l2BridgeMessengerProxyAddress}`);
            } else {
                console.log(`successfully relayed EthDepositMessage on to L2BridgeMessenger with transactionReceipt: ${JSON.stringify(relayEthDepositMessageTxnReceipts[0], bigIntReplacer, 2)}`);
            }
        } catch (err) {
            console.error(`Failed to relay ethBridge message on to L2BridgeMessenger: ${JSON.stringify(err)}`);
        }
    });
});

async function verifyDeploymentCompletion(deployTxn: any, publicClient: PublicClient, contractName: string): Promise<boolean> {

    const deployTxnReceipt = await waitTillCompleted(publicClient, deployTxn.hash, {
        waitTillMainShard: true
    });

    const output = deployTxnReceipt?.[0]?.outputReceipts?.[0];
    if (
        !output?.success ||
        output.status === '' ||
        (typeof output.status === 'string' && output.status.toLowerCase().includes('reverted'))
    ) {
        console.error(`❌ Failed to deploy ${contractName} contract`);
        return false;
    } else {
        console.log(`✅ Successfully deployed ${contractName} contract`);
        return true;
    }
}