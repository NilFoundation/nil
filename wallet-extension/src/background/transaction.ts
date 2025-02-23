import { ERROR_CODES, ERROR_MESSAGES, createError } from "../features/utils/errors.ts";
import {
  type TransactionRequest,
  validateSmartAccountAddress,
  validateTokens,
  validateTransactionFields,
  validateTransactionValue,
} from "../features/utils/transaction.ts";
import { openPopupWindow } from "./popup";
import { getFromStorage, saveToStorage } from "./storage";

type RequestMessage = {
  origin?: string;
  params?: [TransactionRequest];
};

// Updated handleProcessRequest with validations
export async function handleProcessRequest(
  requestId: string,
  request: RequestMessage,
  port: chrome.runtime.Port,
) {
  const origin = request?.origin;

  // Validate origin
  if (!origin) {
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.INVALID_PARAMS, ERROR_MESSAGES.MISSING_ORIGIN),
    });
    return;
  }

  // Check if the website is already connected
  const connectedWebsites = (await getFromStorage("connectedWebsites")) || {};
  if (!connectedWebsites[origin]) {
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.UNAUTHORIZED, ERROR_MESSAGES.UNAUTHORIZED),
    });
    return;
  }

  // Validate request params
  if (!request?.params || !Array.isArray(request.params) || request.params.length !== 1) {
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.INVALID_PARAMS, ERROR_MESSAGES.MISSING_PARAMS),
    });
    return;
  }

  const tx = request.params[0];

  // Validate 'to' address
  if (!tx?.to) {
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.INVALID_PARAMS, ERROR_MESSAGES.INVALID_TO_FIELD),
    });
    return;
  }

  const toValidation = validateSmartAccountAddress(tx.to);
  if (!toValidation.isValid) {
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.INVALID_PARAMS, toValidation.error),
    });
    return;
  }

  // Validate that at least one field exists
  const fieldValidation = validateTransactionFields(tx);
  if (!fieldValidation.isValid) {
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.INVALID_PARAMS, fieldValidation.error),
    });
    return;
  }

  // Validate transaction value
  if (tx?.value !== undefined) {
    const valueValidation = validateTransactionValue(tx.value);
    if (!valueValidation.isValid) {
      port.postMessage({
        requestId,
        error: createError(ERROR_CODES.INVALID_PARAMS, valueValidation.error),
      });
      return;
    }
  }

  // Validate tokens
  if (tx?.tokens !== undefined) {
    const tokensValidation = validateTokens(tx.tokens);
    if (!tokensValidation.isValid) {
      port.postMessage({
        requestId,
        error: createError(ERROR_CODES.INVALID_PARAMS, tokensValidation.error),
      });
      return;
    }
  }

  // Save only necessary fields
  await saveToStorage<TransactionRequest>(`tx_${requestId}`, {
    to: tx.to,
    value: tx.value || 0,
    tokens: tx.tokens || [],
    data: tx.data || null,
  });

  // Open popup window with lightweight parameters
  await openPopupWindow(`/send-sign?requestId=${requestId}&origin=${encodeURIComponent(origin)}`);
}

type SendSignResponseMessage = {
  requestId: string;
  receiptHash?: string;
  origin?: string;
};

// Handle sign and send response
export function handleSendSignResponse(
  message: SendSignResponseMessage,
  port: chrome.runtime.Port,
) {
  const { requestId, receiptHash, origin } = message;

  if (receiptHash && origin) {
    // Return the transaction hash as the result
    port.postMessage({
      requestId,
      result: receiptHash,
    });
  } else {
    // User rejected the sign and send request
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.USER_REJECTED, ERROR_MESSAGES.USER_REJECTED),
    });
  }
}
