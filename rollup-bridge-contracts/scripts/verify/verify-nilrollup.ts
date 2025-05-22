
// npx hardhat run scripts/verify/verify-nilrollup.ts --network sepolia
async function main() {
    // Lazy import inside the function
    // @ts-ignore
    const { run } = await import('hardhat');

    const contractAddress = '';

    try {
        await run('verify:verify', {
            address: contractAddress,
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
