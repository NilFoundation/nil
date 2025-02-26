import { Wallet, ethers } from 'ethers';
import * as dotenv from 'dotenv';
dotenv.config();

// npx ts-node scripts/create-wallet-with-funding.ts
async function createAndUseWallet() {
    const provider = new ethers.JsonRpcProvider('http://localhost:8545'); // Change URL as needed

    const accounts = await provider.send('eth_accounts', []);
    const defaultAccount = accounts[0];

    const wallet = Wallet.createRandom();
    const connectedWallet = wallet.connect(provider);

    const value = ethers.parseEther('1');
    const valueInHex = ethers.toQuantity(ethers.parseEther('1'));

    const fundingTx = await provider.send('eth_sendTransaction', [
        {
            from: defaultAccount,
            to: wallet.address,
            value: valueInHex,
        },
    ]);

    const receivingWallet = Wallet.createRandom();
    console.log('New Wallet Created:', receivingWallet.address);

    const tx = await connectedWallet.sendTransaction({
        to: receivingWallet.address,
        value: ethers.parseEther('0.1'),
        gasLimit: 21000,
    });

    const receipt = await tx.wait();
}

createAndUseWallet().catch((error) => {
    console.error('Error:', error.message);
});
