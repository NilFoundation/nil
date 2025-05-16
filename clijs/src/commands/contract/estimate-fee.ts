import type { EstimateFeeResult, Hex } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { BaseCommand } from "../../base.js";
import { readJsonFile } from "../../common/utils.js";
import { bigintFlag, hexArg } from "../../types.js";

export default class ContractEstimateFee extends BaseCommand {
  static override summary = "Get the recommended fee credit for a transaction";

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
    internal: Flags.boolean({
      char: "i",
      description: "if true, the transaction is internal",
      required: false,
      default: false,
    }),
    deploy: Flags.boolean({
      char: "d",
      description: "if true, the transaction is for deployment",
      required: false,
      default: false,
    }),
  };

  static args = {
    address: hexArg({
      name: "address",
      required: true,
      description: "The address of the smart contract",
    }),
    method: Args.string({
      name: "method",
      required: true,
      description: "The method to call",
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
    const { flags, args } = await this.parse(ContractEstimateFee);
    const { smartAccount } = await this.setupSmartAccount();
    const address = args.address as Hex;

    let abi: Abi;
    try {
      abi = readJsonFile<Abi>(flags.abiPath);
    } catch (e) {
      this.error(`Invalid ABI file: ${e}`);
    }
    const txFlags: string[] = [];
    if (flags.internal) {
      txFlags.push("Internal");
    }
    if (flags.deploy) {
      txFlags.push("Deploy");
    }

    const result: EstimateFeeResult = await smartAccount.client.estimateGas(
      {
        to: address,
        value: flags.amount ?? 0n,
        args: args.args?.split(" ") ?? [],
        abi: abi,
        functionName: args.method,
        flags: txFlags,
      },
      "latest",
    );
    this.log(result.feeCredit.toString());
    return result.feeCredit.toString();
  }
}
