import fs from "node:fs";
import path from "node:path";
import { calculateAddress, hexToBytes, refineSalt } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { bytesToHex, encodeDeployData } from "viem";
import { BaseCommand } from "../../base.js";

export default class ContractAddress extends BaseCommand {
  static override description = "Calculate the address of a smart contract";

  static override examples = ["<%= config.bin %> <%= command.id %> ./contract.sol 'arg1'"];

  static flags = {
    abiPath: Flags.string({
      char: "a",
      description: "The path to the ABI file",
      required: true,
      default: "",
    }),
    binPath: Flags.string({
      char: "b",
      description: "The path to the bin file",
      required: true,
      default: "",
    }),
    salt: Flags.integer({
      char: "s",
      description: "The salt for the deployment transaction",
      required: false,
      default: 1,
    }),
    shardId: Flags.integer({
      char: "h",
      description: "Specify the shard ID to interact with",
      required: false,
      default: 1,
    }),
  };

  static strict = false;

  static args = {
    args: Args.string(),
  };

  async run(): Promise<string> {
    const { argv, flags } = await this.parse(ContractAddress);

    const abiFullPath = path.resolve(flags.abiPath);
    const abiFileContent = fs.readFileSync(abiFullPath, "utf8");
    const abi: Abi = JSON.parse(abiFileContent);

    const binFullPath = path.resolve(flags.binPath);
    const binFileContent = fs.readFileSync(binFullPath, "utf8");

    const constructorData = hexToBytes(
      encodeDeployData({
        abi: abi,
        bytecode: binFileContent as `0x${string}`,
        args: argv || [],
      }),
    );

    const address = calculateAddress(
      flags.shardId,
      constructorData,
      refineSalt(BigInt(flags.salt as number)),
    );
    const hexAddress = bytesToHex(address);
    this.log(hexAddress);
    return hexAddress;
  }
}
