import { ethers } from "ethers";

function getFunctionSelector(signature: string): string {
  // Hash the function signature using ethers
  const hash = ethers.keccak256(ethers.toUtf8Bytes(signature));
  // Take the first 4 bytes of the hash
  const functionSelector = hash.slice(2, 10); // Remove '0x' prefix and take the first 8 characters
  return functionSelector;
}

// npx ts-node scripts/get-function-selector.ts
const signature = "proofBatch(bytes32,bytes32,bytes,uint256)";
const selector = getFunctionSelector(signature);
console.log(selector);
