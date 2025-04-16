import type { CallRes, Hex } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { BaseCommand } from "../../base.js";
import { readJsonFile } from "../../common/utils.js";
import { hexArg } from "../../types.js";

export default class SmartAccountCallReadOnly extends BaseCommand {
  static override summary = "Call view method of field of a smart account";
  static override description =
    "Perform a read-only call to the smart contract with the given address and calldata";

  static flags = {
    abiPath: Flags.string({
      char: "a",
      description: "The path to the ABI file",
      required: true,
    }),
  };

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "The address of the smart contract",
    }),
    bytecodeOrMethod: Args.string({
      name: "bytecodeOrMethod",
      required: true,
      description: "The bytecode or method to send",
    }),
    args: Args.string({
      name: "args",
      required: false,
      description: "Arguments for the method",
      multiple: true,
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<unknown> {
    const { flags, args } = await this.parse(SmartAccountCallReadOnly);
    const { smartAccount } = await this.setupSmartAccount();
    const address = args.address as Hex;
    let abi: Abi;
    try {
      abi = readJsonFile<Abi>(flags.abiPath);
    } catch (e) {
      this.error(`Invalid ABI file: ${e}`);
    }

    let result: CallRes;

    if (args.bytecodeOrMethod.startsWith("0x")) {
      const data = args.bytecodeOrMethod as Hex;
      result = await smartAccount.client.call(
        {
          to: address,
          data,
        },
        "latest",
      );
    } else {
      result = await smartAccount.client.call(
        {
          to: address,
          functionName: args.bytecodeOrMethod,
          abi,
          args: args.args?.split(" ") ?? [],
        },
        "latest",
      );
    }

    if (args.bytecodeOrMethod.startsWith("0x")) {
      return result.data;
    }
    return result.decodedData;
  }
}
