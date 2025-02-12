import { run } from "hardhat";

//  npx hardhat run scripts/verify/verify-nil-verifier.ts --network sepolia
async function main() {
  const contractAddress = "0x5c7EE797E85E53f6F4Df8fF38E71EbFB1aE564E3"; 
  const constructorArguments: any[] = []; 

  try {
    await run("verify:verify", {
      address: contractAddress,
      constructorArguments: constructorArguments,
    });
    console.log("Contract verified successfully");
  } catch (error) {
    console.error("Verification failed:", error);
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });