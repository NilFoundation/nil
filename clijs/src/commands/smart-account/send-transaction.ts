import fs from "node:fs";
import path from "node:path";
import type { Hex } from "@nilfoundation/niljs";
import { Args, Flags } from "@oclif/core";
import type { Abi } from "abitype";
import { BaseCommand } from "../../base.js";

export default class SmartAccountSendTransaction extends BaseCommand {
  static override summary = "Send a transaction to a smart contract via the smart account";
  static override description =
    "Send a transaction to the smart contract with the specified bytecode or command via the smart account";

  static flags = {
    abiPath: Flags.string({
      char: "a",
      description: "The path to the ABI file",
      required: true,
    }),
    amount: Flags.string({
      char: "m",
      description: "The amount of default tokens to send",
      required: false,
    }),
    noWait: Flags.boolean({
      char: "n",
      description: "Define whether the command should wait for the receipt",
      default: false,
    }),
    feeCredit: Flags.string({
      char: "f",
      description: "The fee credit for transaction processing",
      required: false,
    }),
    priorityFee: Flags.string({
        char: "p",
        description: "Priority fee for transaction processing",
        required: false,
      }),
    maxFeePerGas: Flags.string({
        char: "x",
        description: "Max fee per gas for transaction processing",
        required: false,
      }),
    tokens: Flags.string({
      char: "c",
      description:
        "The custom tokens to transfer in as a map 'tokenId=amount', can be set multiple times",
      multiple: true,
      required: false,
    }),
  };

  static args = {
    address: Args.string({
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

  public async run(): Promise<Hex> {
    const { flags, args } = await this.parse(SmartAccountSendTransaction);

    const { smartAccount } = await this.setupSmartAccount();

    const address = args.address as Hex;
    const abiPath = flags.abiPath;

    const abiFullPath = path.resolve(abiPath);
    const abiFileContent = fs.readFileSync(abiFullPath, "utf8");
    const abi: Abi = JSON.parse(abiFileContent);

    let txHash: Hex;

    const tokens = flags.tokens?.map((token) => {
      const [tokenId, amount] = token.split("=");
      return { id: tokenId as Hex, amount: BigInt(amount) };
    });

    if (args.bytecodeOrMethod.startsWith("0x")) {
      const data = args.bytecodeOrMethod as Hex;
      txHash = await smartAccount.sendTransaction({
        to: address,
        value: BigInt(flags.amount ?? 0),
        feeCredit: BigInt(flags.feeCredit ?? 0),
        priorityFee: BigInt(flags.priorityFee ?? 0),
        maxFeePerGas: BigInt(flags.maxFeePerGas ?? 0),
        tokens: tokens,
        data: data,
      });
    } else {
      txHash = await smartAccount.sendTransaction({
        to: address,
        value: BigInt(flags.amount ?? 0),
        feeCredit: BigInt(flags.feeCredit ?? 0),
        priorityFee: BigInt(flags.priorityFee ?? 0),
        maxFeePerGas: BigInt(flags.maxFeePerGas ?? 0),
        args: args.args?.split(" ") ?? [],
        abi: abi,
        functionName: args.bytecodeOrMethod,
        tokens: tokens,
      });
    }
    if (flags.quiet) {
      this.log(txHash);
    } else {
      this.log(`Transaction hash: ${txHash}`);
    }

    if (flags.noWait) {
      return txHash;
    }

    this.info("Waiting for the transaction to be processed...");
    await this.waitOnTx(txHash);
    this.info("Transaction successfully processed");

    return txHash;
  }
}
