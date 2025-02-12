import { ethers, upgrades } from "hardhat";
import { abi as ProxyAdminABI } from "@openzeppelin/contracts/build/contracts/ProxyAdmin.json";

// npx hardhat run scripts/transfer_proxy_admin_ownership.ts --network sepolia
async function main() {
  // Replace with your deployed proxy address and new owner address
  const proxyAddress = '0xd1A727Ac99ECb5Ab16797b3D585464F56fD7611a';
  const newOwnerAddress = "0x658805a93Af995ccf5C2ab3B9B06302653289E68";

  // Retrieve the ProxyAdmin address
  const proxyAdminAddress = await upgrades.erc1967.getAdminAddress(proxyAddress);
  console.log("ProxyAdmin deployed to:", proxyAdminAddress);

  // Attach to the ProxyAdmin contract using the ABI from OpenZeppelin
  const proxyAdmin = new ethers.Contract(proxyAdminAddress, ProxyAdminABI, ethers.provider);

  // Get the signer
  const [signer] = await ethers.getSigners();

  // Transfer ownership to the new owner
  const tx = await proxyAdmin.connect(signer).transferOwnership(newOwnerAddress);
  await tx.wait();

  console.log("ProxyAdmin ownership transferred to:", newOwnerAddress);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });