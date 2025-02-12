import fs from "node:fs";
import path from "node:path";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { encodeFunctionData } from "viem";
import { BaseCommand } from "../../base.js";

export default class AbiEncode extends BaseCommand {
  static override summary = "Encode a contract call";
  static override description = "Encode a contract call";

  static flags = {
    path: Flags.string({
      char: "p",
      description: "Path to ABI file",
      required: true,
    }),
  };

  static args = {
    method: Args.string({
      name: "method",
      required: true,
      description: "Methods name",
    }),
    args: Args.string({
      name: "args",
      required: true,
      description: "Method args",
      multiple: true,
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<string> {
    const { flags, args } = await this.parse(AbiEncode);

    const abiPath = flags.path;
    const abiFullPath = path.resolve(abiPath);
    const abiFileContent = fs.readFileSync(abiFullPath, "utf8");
    const abi: Abi = JSON.parse(abiFileContent);

    return encodeFunctionData({
      abi: abi,
      functionName: args.method,
      args: args.args?.split(" ") ?? [],
    });
  }
}
