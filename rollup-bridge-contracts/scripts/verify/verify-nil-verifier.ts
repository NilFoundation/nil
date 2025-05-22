//  npx hardhat run scripts/verify/verify-nil-verifier.ts --network sepolia
async function main() {
    // Lazy import inside the function
    // @ts-ignore
    const { run } = await import('hardhat');

    const contractAddress = '';
    const constructorArguments: any[] = [];

    try {
        await run('verify:verify', {
            address: contractAddress,
            constructorArguments: constructorArguments,
        });
    } catch (error) {
        console.error('Verification failed:', error);
    }
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });
