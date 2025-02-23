import { ERROR_MESSAGES } from "./errors";
import type { ValidationResult } from "./inputValidation";

export type TransactionRequest = {
  to: string;
  value?: number;
  tokens?: { id: string; amount: number }[];
  data?: string | null;
};

// Validate Smart Account Address
export const validateSmartAccountAddress = (smartAccountAddress: string): ValidationResult => {
  const shardNumber = import.meta.env.VITE_NUMBER_SHARDS;
  const isValidLength = smartAccountAddress.length === 42;
  const isHex = /^0x[a-fA-F0-9]{40}$/.test(smartAccountAddress);
  const validPrefixes = Array.from(
    { length: shardNumber },
    (_, i) => `0x${(i + 1).toString().padStart(4, "0")}`,
  );
  const hasValidPrefix = validPrefixes.some((prefix) => smartAccountAddress.startsWith(prefix));

  if (isValidLength && isHex && hasValidPrefix) {
    return { isValid: true, error: "" };
  }

  return { isValid: false, error: ERROR_MESSAGES.INVALID_SMART_ACCOUNT };
};

// Validate Transaction Value
export const validateTransactionValue = (value: number): ValidationResult => {
  if (Number.isNaN(value)) {
    return { isValid: false, error: ERROR_MESSAGES.INVALID_VALUE };
  }
  if (value <= 0) {
    return { isValid: false, error: ERROR_MESSAGES.VALUE_TOO_LOW };
  }
  return { isValid: true, error: "" };
};

type Token = {
  id: string;
  amount: number;
};

// Validate Tokens Array
export const validateTokens = (tokens: Token[]): ValidationResult => {
  if (!Array.isArray(tokens)) {
    return { isValid: false, error: ERROR_MESSAGES.INVALID_TOKEN_ARRAY };
  }

  for (const token of tokens) {
    if (!/^0x[a-fA-F0-9]{40}$/.test(token.id)) {
      return { isValid: false, error: ERROR_MESSAGES.INVALID_TOKEN_ID(token.id) };
    }

    if (typeof token.amount !== "number" || token.amount <= 0) {
      return { isValid: false, error: ERROR_MESSAGES.INVALID_TOKEN_AMOUNT(token.id) };
    }

    if (!Number.isInteger(token.amount)) {
      return { isValid: false, error: ERROR_MESSAGES.DECIMAL_TOKEN_AMOUNT(token.id) };
    }
  }

  return { isValid: true, error: "" };
};

export const validateTransactionFields = (tx: TransactionRequest) => {
  const hasValue = tx?.value !== undefined && tx.value > 0;
  const hasTokens = tx?.tokens && Array.isArray(tx.tokens) && tx.tokens.length > 0;
  const hasData = tx?.data !== undefined && tx.data !== null;

  if (!hasValue && !hasTokens && !hasData) {
    return { isValid: false, error: ERROR_MESSAGES.MISSING_TRANSACTION_FIELDS };
  }

  return { isValid: true, error: "" };
};
