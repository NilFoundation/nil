import { Wallet, ethers } from 'ethers';
import * as dotenv from 'dotenv';
dotenv.config();

// npx ts-node scripts/fund-wallet.ts
async function createAndUseWallet() {
    const GETH_RPC_ENDPOINT = process.env.GETH_RPC_ENDPOINT as string;
    console.log('L1 RPC Endpoint:', GETH_RPC_ENDPOINT);
    const provider = new ethers.JsonRpcProvider(GETH_RPC_ENDPOINT);

    const accounts = await provider.send('eth_accounts', []);
    const defaultAccount = accounts[0];

    const valueInHex = ethers.toQuantity(ethers.parseEther('100'));

    const walletAddress = process.env.GETH_WALLET_ADDRESS as string;

    const fundingTx = await provider.send('eth_sendTransaction', [
        {
            from: defaultAccount,
            to: walletAddress,
            value: valueInHex,
        },
    ]);

    const transactionHash = fundingTx;

    const receipt = await provider.waitForTransaction(transactionHash);
    return receipt;
}

createAndUseWallet().catch((error) => {
    console.error('Error:', error.message);
});
