import type { Hex, SmartAccountV1, Token } from "@nilfoundation/niljs";
import { hexToBigInt, toHex } from "@nilfoundation/niljs";
import { formatEther, parseEther } from "viem";
import { estimateFee } from "../blockchain/smartAccount.ts";
import { Currency } from "../components/currency";

export const MIN_SEND_AMOUNT_NIL = 0.00001;
export const MIN_SEND_AMOUNT_OTHER = 1;

// Validate send amount
export function validateSendAmount(
  amount: string,
  selectedCurrency: string,
  balance: bigint,
): string | null {
  if (!amount.trim()) return "Please enter an amount";

  const numericAmount = Number(amount);
  if (Number.isNaN(numericAmount)) return "Invalid input. Please enter a valid number";

  return selectedCurrency === Currency.NIL
    ? validateNilSendAmount(numericAmount, Number(balance), amount)
    : validateOtherCurrencySendAmount(numericAmount, selectedCurrency, Number(balance));
}

function validateNilSendAmount(
  amount: number,
  balance: number,
  stringAmount: string,
): string | null {
  if (parseEther(stringAmount) > balance) return "Insufficient Funds";
  if (amount < MIN_SEND_AMOUNT_NIL) return `Minimum send amount is ${MIN_SEND_AMOUNT_NIL} NIL`;
  return null;
}

function validateOtherCurrencySendAmount(
  amount: number,
  currency: string,
  balance: number,
): string | null {
  if (amount > balance) return "Insufficient Funds";
  if (amount < MIN_SEND_AMOUNT_OTHER)
    return `Minimum send amount is ${MIN_SEND_AMOUNT_OTHER} ${currency}`;
  if (!Number.isInteger(amount)) return `${currency} does not support decimal values`;
  return null;
}

// Estimate transaction fee
export async function fetchEstimatedFee(
  smartAccount: SmartAccountV1,
  toAddress: Hex,
  amount: string,
  tokenAddress: string,
) {
  const transactionTokens: Token[] =
    tokenAddress === ""
      ? []
      : [{ id: tokenAddress as Hex, amount: hexToBigInt(toHex(Number(amount))) }];
  const value = tokenAddress === "" ? parseEther(amount) : 0n;

  try {
    const fee = await estimateFee(smartAccount, toAddress, value, transactionTokens);
    return formatEther(fee);
  } catch (error) {
    console.error("Failed to estimate fee:", error);
    throw Error("Error estimating fee");
  }
}
