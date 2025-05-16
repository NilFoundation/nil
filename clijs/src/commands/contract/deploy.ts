import {
  type Hex,
  bytesToHex,
  externalDeploymentTransaction,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { BaseCommand } from "../../base.js";
import { readFile, readJsonFile } from "../../common/utils.js";
import { bigintFlag, hexArg } from "../../types.js";

export default class ContractDeploy extends BaseCommand {
  static override summary = "Deploy a smart contract";
  static override description =
    "Deploy the smart contract with the specified hex-bytecode from stdin or from file";

  static flags = {
    shardId: Flags.integer({
      char: "s",
      description: "Specify the shard ID to interact with",
      required: false,
      default: 1,
    }),
    salt: Flags.integer({
      char: "t",
      description: "The salt for the deploy transaction",
      required: false,
      default: 0,
    }),
    abiPath: Flags.string({
      char: "a",
      description: "The path to the ABI file",
      required: true,
    }),
    feeCredit: bigintFlag({
      char: "f",
      description: "The fee credit for transaction processing",
      required: false,
      default: 10000000000n,
    }),
    noWait: Flags.boolean({
      char: "n",
      description: "Define whether the command should wait for the receipt",
      required: false,
      default: false,
    }),
  };

  static args = {
    filename: hexArg({
      name: "filename",
      required: true,
      description: "Bytecode file name",
    }),
    method: Args.string({
      name: "method",
      required: true,
      description: "Contract methods",
    }),
    args: Args.string({
      name: "args",
      required: false,
      description: "Method arguments",
      multiple: true,
    }),
  };

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<Hex> {
    const { flags, args } = await this.parse(ContractDeploy);

    const abi = readJsonFile<Abi>(flags.abiPath);
    const bytecode = readFile(args.filename);

    if (!this.rpcClient) {
      throw new Error("RPC client is not initialized");
    }

    const chainId = await this.rpcClient.chainId();
    const gasPrice = await this.rpcClient.getGasPrice(flags.shardId);

    const deploymentTransaction = externalDeploymentTransaction(
      {
        salt: BigInt(flags.salt),
        shard: flags.shardId,
        bytecode: bytecode as `0x${string}`,
        abi: abi,
        args: args.args?.split(" ") ?? [],
        feeCredit: flags.feeCredit * gasPrice,
      },
      chainId,
    );
    const address = bytesToHex(deploymentTransaction.to);

    const hash = await deploymentTransaction.send(this.rpcClient);
    if (!flags.noWait) {
      await waitTillCompleted(this.rpcClient, hash);
    }
    this.log(`Transaction hash: ${hash}`);
    return address;
  }
}
