import { CometaService, HttpTransport, getShardIdFromAddress } from "@nilfoundation/niljs";
import { scope } from "hardhat/config";
import type { HardhatRuntimeEnvironment } from "hardhat/types";
import { getValue } from "../config";

const cometaTask = scope("cometa", "Cometa service");

cometaTask
  .task("register", "Register contract in cometa service")
  .addParam("address", "Address of the contract")
  .addParam("contract", "Contract name")
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    await (hre as any).registerContract(taskArgs.contract, taskArgs.address);
    console.log("Register in Cometa successfully");
  });

cometaTask
  .task("info", "Info about the contract")
  .addParam("address", "Address of the contract")
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    const endpoint = getValue("cometa_endpoint");
    if (!endpoint) {
      throw Error("Cometa endpoint is not set");
    }
    const cometa = new CometaService({
      transport: new HttpTransport({
        endpoint: endpoint,
      }),
      shardId: getShardIdFromAddress(taskArgs.address),
    });
    const contract = await cometa.getContract(taskArgs.address);
    console.log(contract);
  });
