import type { Abi, Address } from "abitype";
import { type EncodeFunctionDataParameters, decodeFunctionResult, encodeFunctionData } from "viem";
import type { PublicClient } from "../clients/PublicClient.js";
import { ExternalTransactionEnvelope } from "../encoding/externalTransaction.js";
import { bytesToHex } from "../encoding/fromBytes.js";
import { hexToBytes } from "../encoding/fromHex.js";
import type { ISigner } from "../signers/types/ISigner.js";
import type {
  SendTransactionParams,
  SmartAccountInterface,
} from "../smart-accounts/SmartAccountInterface.js";
import type { Hex } from "../types/Hex.js";
import type { ReadContractReturnType } from "../types/utils.js";
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
  Pick<SendTransactionParams, "feeCredit" | "seqno" | "tokens" | "value" | "to">
>;

export async function writeContract<
  const abi extends Abi | readonly unknown[],
  functionName extends ContractFunctionName<abi, "payable" | "nonpayable">,
  const args extends ContractFunctionArgs<abi, "payable" | "nonpayable", functionName>,
>({
  smartAccount,
  args,
  abi,
  functionName,
  options,
}: {
  smartAccount: SmartAccountInterface;
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
  const tx = await smartAccount.sendTransaction({
    data: calldata,
    deploy: false,
    ...options,
  });
  return tx.hash;
}

export async function writeExternalContract<
  const abi extends Abi | readonly unknown[],
  functionName extends ContractFunctionName<abi, "payable" | "nonpayable">,
  const args extends ContractFunctionArgs<abi, "payable" | "nonpayable", functionName>,
>({
  client,
  signer,
  args,
  abi,
  functionName,
  options,
}: {
  client: PublicClient;
  signer: ISigner;
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

  const toBytes = options.to instanceof Uint8Array ? options.to : hexToBytes(options.to as Address);
  const toAddress =
    options.to instanceof Uint8Array ? bytesToHex(options.to) : (options.to as Address);

  const [refinedSeqno, chainId] = await Promise.all([
    client.getTransactionCount(toAddress, "latest"),
    client.chainId(),
  ]);

  const transaction = new ExternalTransactionEnvelope({
    isDeploy: false,
    to: toBytes,
    chainId: chainId,
    seqno: refinedSeqno,
    data: hexToBytes(calldata),
    authData: new Uint8Array(0),
  });
  transaction.authData = await transaction.sign(signer);
  const encodedTransaction = transaction.encode();
  return await client.sendRawTransaction(bytesToHex(encodedTransaction));
}
