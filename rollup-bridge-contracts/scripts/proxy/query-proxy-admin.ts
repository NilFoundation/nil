import { ethers, upgrades } from 'hardhat';

// npx hardhat run scripts/proxy/query-proxy-admin.ts --network sepolia
async function main() {
    // Retrieve and log the ProxyAdmin address
    const proxyAddress = '';
    const proxyAdminAddress =
        await upgrades.erc1967.getAdminAddress(proxyAddress);
    console.log(
        `ProxyAdmin for proxy: ${proxyAddress} is: ${proxyAdminAddress}`,
    );

    // Retrieve and log the implementation address
    const implementationAddress =
        await upgrades.erc1967.getImplementationAddress(proxyAddress);
    console.log(
        `Implementation address for proxy: ${proxyAddress} is: ${implementationAddress}`,
    );
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });
