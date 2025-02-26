import { run } from 'hardhat';

//  npx hardhat run scripts/verify/verify-nil-verifier.ts --network sepolia
async function main() {
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
