import { ethers, upgrades } from 'hardhat';
import { abi as ProxyAdminABI } from '@openzeppelin/contracts/build/contracts/ProxyAdmin.json';

// npx hardhat run scripts/transfer_proxy_admin_ownership.ts --network sepolia
async function main() {
    // Replace with your deployed proxy address and new owner address
    const proxyAddress = '';
    const newOwnerAddress = '';

    // Retrieve the ProxyAdmin address
    const proxyAdminAddress =
        await upgrades.erc1967.getAdminAddress(proxyAddress);
    console.log('ProxyAdmin deployed to:', proxyAdminAddress);

    // Attach to the ProxyAdmin contract using the ABI from OpenZeppelin
    const proxyAdmin = new ethers.Contract(
        proxyAdminAddress,
        ProxyAdminABI,
        ethers.provider,
    );

    const [signer] = await ethers.getSigners();

    const tx = await proxyAdmin
        .connect(signer)
        .transferOwnership(newOwnerAddress);
    await tx.wait();

    console.log('ProxyAdmin ownership transferred to:', newOwnerAddress);
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });
