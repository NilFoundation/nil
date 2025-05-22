// npx hardhat run scripts/proxy/query-proxy-admin.ts --network sepolia
async function main() {
    // Lazy import inside the function
    // @ts-ignore
    const { upgrades } = await import('hardhat');

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
