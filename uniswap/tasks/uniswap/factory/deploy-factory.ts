import type { Abi } from "abitype";
import { task } from "hardhat/config";
import FactoryJson from "../../../artifacts/contracts/UniswapV2Factory.sol/UniswapV2Factory.json";
import { createWallet } from "../../basic/basic";
import { deployNilContract } from "../../util/deploy";

task("deploy-factory").setAction(async (taskArgs, _) => {
  const wallet = await createWallet();

  const { contract, address } = await deployNilContract(
    wallet,
    FactoryJson.abi as Abi,
    FactoryJson.bytecode,
    [wallet.address],
    wallet.shardId,
    [],
  );
  console.log("Uniswap factory contract deployed at address: " + address);
});
