import { run } from "hardhat";

// npx hardhat run scripts/verify/verify-nilrollup.ts --network sepolia
async function main() {
  const contractAddress = "0x796baf7E572948CD0cbC374f345963bA433b47a2"; 

  try {
    await run("verify:verify", {
      address: contractAddress,
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