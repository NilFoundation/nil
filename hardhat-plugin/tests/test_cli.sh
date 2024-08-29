set -e

HARDHAT_IGNITION_CONFIRM_DEPLOYMENT=false npx hardhat ignition deploy ./ignition/modules/Incrementer.ts --network nil 2>/dev/null

# todo: fetch the address of the deployed contract and use it to test tasks
