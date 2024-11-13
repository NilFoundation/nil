import type { Abi } from "abitype";
import { type EncodeFunctionDataParameters, decodeFunctionResult, encodeFunctionData } from "viem";
import type { PublicClient } from "../clients/index.js";
import type { Hex } from "../types/index.js";
import type { ReadContractReturnType } from "../types/utils.js";
import type { SendMessageParams, WalletInterface } from "../wallets/WalletInterface.js";
import type {
  ContractFunctionArgs,
  ContractFunctionName,
  ReadContractParameters,
} from "./ContractFactory.js";

export async function contractInteraction<
  const abi extends Abi | readonly unknown[],
  fN extends ContractFunctionName<abi, "pure" | "view">,
  const args extends ContractFunctionArgs<abi, "pure" | "view", fN>,
>(
  client: PublicClient,
  parameters: ReadContractParameters<abi, fN, args>,
): Promise<ReadContractReturnType<abi, fN, args>> {
  const { abi, to, args, functionName } = parameters;
  const calldata: Hex = encodeFunctionData({
    abi,
    args,
    functionName,
  } as EncodeFunctionDataParameters);

  const result = await client.call(
    {
      data: calldata,
      to: to,
    },
    "latest",
  );

  // @ts-ignore
  return decodeFunctionResult({
    abi: abi,
    data: result.data,
    functionName,
    args,
  }) as ReadContractReturnType<abi, fN, args>;
}

export type WriteOptions = Partial<
  Pick<SendMessageParams, "feeCredit" | "seqno" | "tokens" | "value" | "to">
>;

export async function writeContract<
  const abi extends Abi | readonly unknown[],
  functionName extends ContractFunctionName<abi, "payable" | "nonpayable">,
  const args extends ContractFunctionArgs<abi, "payable" | "nonpayable", functionName>,
>({
  wallet,
  args,
  abi,
  functionName,
  options,
}: {
  wallet: WalletInterface;
  args: args;
  abi: abi;
  functionName: functionName;
  options: WriteOptions;
}): Promise<Hex> {
  const calldata = encodeFunctionData({
    abi,
    args,
    functionName,
  } as EncodeFunctionDataParameters);
  // @ts-ignore
  const hex = await wallet.sendMessage({
    data: calldata,
    deploy: false,
    ...options,
  });
  return hex;
}
