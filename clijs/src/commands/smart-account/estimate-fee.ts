import type { EstimateFeeResult, Hex } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { BaseCommand } from "../../base.js";
import { readJsonFile } from "../../common/utils.js";
import { bigintFlag, hexArg } from "../../types.js";

export default class SmartAccountEstimateFee extends BaseCommand {
  static override summary = "Get the recommended fees";
  static override description =
    "Get the recommended fees (internal and external) for a transaction sent by the smart account";

  static flags = {
    abiPath: Flags.string({
      char: "a",
      description: "The path to the ABI file",
      required: true,
    }),
    amount: bigintFlag({
      char: "m",
      description: "The amount of default tokens to send",
      required: false,
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

  public async run(): Promise<string> {
    const { flags, args } = await this.parse(SmartAccountEstimateFee);
    const { smartAccount } = await this.setupSmartAccount();
    const address = args.address as Hex;

    let abi: Abi;
    try {
      abi = readJsonFile<Abi>(flags.abiPath);
    } catch (e) {
      this.error(`Invalid ABI file: ${e}`);
    }

    let result: EstimateFeeResult;

    if (args.bytecodeOrMethod.startsWith("0x")) {
      const data = args.bytecodeOrMethod as Hex;
      result = await smartAccount.client.estimateGas(
        {
          to: address,
          value: flags.amount ?? 0n,
          data: data,
        },
        "latest",
      );
    } else {
      result = await smartAccount.client.estimateGas(
        {
          to: address,
          value: flags.amount ?? 0n,
          args: args.args?.split(" ") ?? [],
          abi: abi,
          functionName: args.bytecodeOrMethod,
        },
        "latest",
      );
    }

    return result.feeCredit.toString();
  }
}
