import { BaseCommand } from "../../base.js";
import { Args, Flags } from "@oclif/core";
import fs from "node:fs";
import path from "node:path";
import { ConfigKeys } from "../../common/config.js";
import { type Hex, LocalECDSAKeySigner, waitTillCompleted, WalletV1 } from "@nilfoundation/niljs";
import type { Abi } from "abitype";

export default class WalletSendMessage extends BaseCommand {
  static override summary = "Send a message to a smart contract via the wallet";
  static override description =
    "Send a message to the smart contract with the specified bytecode or command via the wallet";

  static flags = {
    abiPath: Flags.string({
      char: "a",
      description: "The path to the ABI file",
      required: false,
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

    const privateKey = this.cfg?.[ConfigKeys.PrivateKey] as Hex;
    if (!privateKey) {
      this.error("Private key not found in config. Perhaps you need to run 'keygen new' first?");
    }

    const walletAddress = this.cfg?.[ConfigKeys.Address] as Hex;
    if (!walletAddress) {
      this.error("Address not found in config. Perhaps you need to run 'wallet new' first?");
    }

    const signer = new LocalECDSAKeySigner({
      privateKey: privateKey,
    });

    const pubkey = signer.getPublicKey();
    const wallet = new WalletV1({
      pubkey: pubkey,
      address: walletAddress,
      client:
        this.rpcClient ??
        (() => {
          throw new Error("RPC client is not initialized");
        })(),
      signer,
    });

    const address = args.address as Hex;
    const abiPath = flags.abiPath;

    let abi: Abi;
    if (abiPath) {
      const abiFullPath = path.resolve(abiPath);
      const abiFileContent = fs.readFileSync(abiFullPath, "utf8");
      abi = JSON.parse(abiFileContent);
    } else {
      this.error("ABI path is required to send a message to the smart contract");
    }

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
    if (flags.noWait) {
      if (flags.quite) {
        this.log(txHash);
      } else {
        this.log(`Message hash: ${txHash}`);
      }
      return txHash;
    }
    if (!flags.quite) {
      this.log("Waiting for the message to be processed...");
    }
    const receipt = await waitTillCompleted(this.rpcClient, txHash);
    this.log("Receipts: ", receipt);
    return txHash;
  }
}
