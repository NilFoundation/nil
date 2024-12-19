import { BaseCommand } from "../../base.js";
import { Args, Flags } from "@oclif/core";
import fs from "node:fs";
import path from "node:path";
import type { Hex } from "@nilfoundation/niljs";
import type { Abi } from "abitype";

export default class WalletSendMessage extends BaseCommand {
  static override summary = "Send a message to a smart contract via the wallet";
  static override description =
    "Send a message to the smart contract with the specified bytecode or command via the wallet";

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
      description: "The fee credit for message processing",
      required: false,
    }),
    currencies: Flags.string({
      char: "c",
      description:
        "The custom currencies to transfer in as a map 'currencyId=amount', can be set multiple times",
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
    const { flags, args } = await this.parse(WalletSendMessage);

    const { wallet } = await this.setupWallet();

    const address = args.address as Hex;
    const abiPath = flags.abiPath;

    const abiFullPath = path.resolve(abiPath);
    const abiFileContent = fs.readFileSync(abiFullPath, "utf8");
    const abi: Abi = JSON.parse(abiFileContent);

    let txHash: Hex;

    const tokens = flags.currencies?.map((currency) => {
      const [currencyId, amount] = currency.split("=");
      return { id: currencyId as Hex, amount: BigInt(amount) };
    });

    if (args.bytecodeOrMethod.startsWith("0x")) {
      const data = args.bytecodeOrMethod as Hex;
      txHash = await wallet.sendMessage({
        to: address,
        value: BigInt(flags.amount ?? 0),
        feeCredit: BigInt(flags.feeCredit ?? 0),
        tokens: tokens,
        data: data,
      });
    } else {
      txHash = await wallet.sendMessage({
        to: address,
        value: BigInt(flags.amount ?? 0),
        feeCredit: BigInt(flags.feeCredit ?? 0),
        args: args.args?.split(" ") ?? [],
        abi: abi,
        functionName: args.bytecodeOrMethod,
        tokens: tokens,
      });
    }

    if (flags.quiet) {
      this.log(txHash);
    } else {
      this.log(`Message hash: ${txHash}`);
    }

    if (flags.noWait) {
      return txHash;
    }

    this.info("Waiting for the message to be processed...");
    await this.waitOnTx(txHash);
    this.info("Message successfully processed");

    return txHash;
  }
}
