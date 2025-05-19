import type { Hex } from "@nilfoundation/niljs";
import { bytesToHex } from "viem";
import { BaseCommand } from "../../base.js";
import { hexArg } from "../../types.js";

export default class ContractCode extends BaseCommand {
  static override summary = "Get the code of a smart contract";

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "The address of the smart contract",
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %> 0x1234567890abcdef"];

  public async run(): Promise<string> {
    const { args } = await this.parse(ContractCode);
    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }
    const client = this.rpcClient;
    const address = args.address as Hex;

    const code = await client.getCode(address, "latest");
    const hexCode = bytesToHex(code);

    this.log(hexCode);

    return hexCode;
  }
}
