
import { createAndUseWallet } from "../wallet/create-wallet-with-funding";
import { generateNilSmartAccount } from "../../task/nil-smart-account";
import { bridgeETHAux } from "./bridge-eth-impl";
import { validateL2EthBridging } from "../../task/validate-l2-eth-bridging";
import { fetchRelayer } from "../../task/fetch-relayeraddress-from-relayer";
import { withdrawETH } from "./withdraw-eth";
import { ethers } from "ethers";
import { 
    FaucetClient, 
    HttpTransport,
    PublicClient,
    convertEthToWei,
    waitTillCompleted,
    Hex
 } from "@nilfoundation/niljs";

export interface BridgeLoadGeneratorParams {
    l1Endpoint: string;
    l2Network: string;
    l2Endpoint: string;   
    relayerEndpoint: string;
    syncCommitteeL1AccountAddress: string;
} 

export async function depositAndWithdrawLoop(params: BridgeLoadGeneratorParams) {

    // prerequisites:
    // 1. Ensure the L1 and L2 networks are running.
    // 2. Ensure rollup and bridge contracts are deployed on both L1 and L2 networks
    // 3. Ensure that relayer is up and running
    // 4. Ensure that sync committee is up and running

    // Loop struct:
    // - check balances:
    // - if L1 account balance is low - fund it
    // - if sync committee L1 balance is low - fund it
    // - if relayer L2 account balance is low - fund it    
    // - deposit some amount of ETH/ERC20 from L1 to L2
    // - wait for the deposit to be confirmed on L2

    // - withdraw the deposited amount back to L1
    // - wait for the withdrawal to be confirmed on L1
    // - deposit some ETH/ERC20 to L2
    // - wait for the deposit to be received by L2 account
    // - withdraw the deposited amount back to L1
    // - wait for the withdrawal to be confirmed on L1


    let relayerSmartAccountAddress = await fetchRelayer(params.relayerEndpoint) as Hex;
    console.log("Relayer Smart Account Address:", relayerSmartAccountAddress);

    let l1Wallet = await createAndUseWallet(params.l1Endpoint);
    console.log("L1 Testing Wallet Address:", l1Wallet);

    let [adminL2Account, testingL2Account] = await generateNilSmartAccount(params.l2Network);
    console.log("L2 Testing Smart Account Address:", testingL2Account.address);

    await fundL1Account(params.l1Endpoint, l1Wallet.address, '100')
    await fundL2Account(params.l2Endpoint, testingL2Account.address, '100');
 

    const l2Client = new PublicClient({
        transport: new HttpTransport({ endpoint: params.l2Endpoint }),
    });

    while (true) {
        let l1AccountBalanceLow = false;        // TODO fund if low
        let syncCommitteeL1BalanceLow = false;  // TODO fund if low
        let relayerL2AccountBalanceLow = false; // TODO fund if low      
        
        {
            let relayerL2AccountBalance = await l2Client.getBalance(relayerSmartAccountAddress);
            console.log(`Relayer L2 account balance: ${relayerL2AccountBalance}`);
        }

        {
            let balanceL2 = await l2Client.getBalance(testingL2Account.address);
            let balanceL1 = await l1Wallet.provider.getBalance(l1Wallet.address)

            console.log(`Current balances L2: ${balanceL2} L1: ${balanceL1}`);

            const weiAmount = 1000_000_000_000_000; // 0.001 ETH

            const l1DepositMessageHash = await bridgeETHAux("geth", l1Wallet, testingL2Account.address, BigInt(weiAmount));
            console.log(`L1 Deposit Message Hash: ${l1DepositMessageHash}`);

            await validateL2EthBridging(params.l2Network, testingL2Account, BigInt(weiAmount), l1DepositMessageHash);

            balanceL2 = await testingL2Account.client.getBalance(testingL2Account.address);

            console.log("L2 account balance after deposit:", balanceL2);  

            await withdrawETH("local", testingL2Account, l1Wallet.address, BigInt(weiAmount)); 

            // TODO checks that funds are back on L1 account
        }
    }

    console.log("Deposit and withdraw loop exit");
}


// for l1 recipient account && sync committee account
async function fundL1Account(rpcEndpoint: string, walletAddress: string, amount: string) {
    const provider = new ethers.JsonRpcProvider(rpcEndpoint);

    const accounts = await provider.send('eth_accounts', []);
    const defaultAccount = accounts[0];

    const valueInHex = ethers.toQuantity(ethers.parseEther(amount));;

    const fundingTx = await provider.send('eth_sendTransaction', [
        {
            from: defaultAccount,
            to: walletAddress,
            value: valueInHex,
        },
    ]);

    const transactionHash = fundingTx;
    const receipt = await provider.waitForTransaction(transactionHash);
    const balance = await provider.getBalance(walletAddress);

    console.log(`Funded L1 account ${walletAddress} with ${amount} ETH, current balance ${balance}. Transaction Hash: ${transactionHash}`);
}

// for l2 receipient account && relayer account
async function fundL2Account(rpcEndpoint: string, smartAccountAddress: Hex, amount: string) {

    const faucetClient = new FaucetClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });

    console.log(`topping up L2 account ${smartAccountAddress} with ${amount} ETH via faucet`);

    const topUpFaucet = await faucetClient.topUp({
        smartAccountAddress: smartAccountAddress,
        amount: convertEthToWei(0.1),
        faucetAddress: process.env.NIL as `0x${string}`, // TODO
    });

    const client = new PublicClient({
        transport: new HttpTransport({ endpoint: rpcEndpoint }),
    });

    await waitTillCompleted(client, topUpFaucet);

    const balance = await client.getBalance(smartAccountAddress);
    console.log(`Funded L2 account ${smartAccountAddress} with ${amount} ETH, current balance ${balance}.`);
}


depositAndWithdrawLoop({
    l1Endpoint: process.env.GETH_RPC_ENDPOINT as string,
    l2Network: "local",
    l2Endpoint: process.env.NIL_RPC_ENDPOINT as string,
    relayerEndpoint: process.env.RELAYER_RPC_URL as string,
} as BridgeLoadGeneratorParams).then(() => {
    console.log('Deposit and withdraw loop execution finished');
})
