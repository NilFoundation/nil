export const ERROR_CODES = {
  INVALID_PARAMS: 32602,
  USER_REJECTED: 4001,
  UNAUTHORIZED: 4100,
  UNSUPPORTED_METHOD: 4200,
  PROVIDER_DISCONNECTED: 4900,
  CHAIN_DISCONNECTED: 4901,
  INTERNAL_ERROR: -32603,
};

export const ERROR_MESSAGES = {
  MISSING_ORIGIN: "Missing origin URL",
  USER_REJECTED: "User rejected the request",
  UNAUTHORIZED: "Unauthorized: Account is not connected",
  INVALID_SMART_ACCOUNT: "Invalid smartAccount address",
  INVALID_VALUE: "Invalid value. Please enter a valid number",
  VALUE_TOO_LOW: "Transaction value must be greater than zero",
  INVALID_TOKEN_ARRAY: "Tokens must be an array",
  MISSING_PARAMS: `'params' field is required and must be a single-item array`,
  INVALID_TO_FIELD: "'to' field is required and must be a valid string",
  MISSING_TRANSACTION_FIELDS: "At least one of 'value', 'tokens', or 'data' must be provided",
  INVALID_TOKEN_ID: (id: string) => `Invalid token ID: ${id}`,
  INVALID_TOKEN_AMOUNT: (id: string) => `Invalid token amount for ID ${id}`,
  UNSUPPORTED_METHOD: (method: string) => `Unsupported method: ${method}`,
  DECIMAL_TOKEN_AMOUNT: (id: string) => `Token amount for ${id} cannot be a decimal value`,
};

// Utility to create structured errors
export const createError = (code: number, message: string) => ({
  code,
  message,
});
