import { ERROR_CODES, ERROR_MESSAGES, createError } from "../features/utils/errors.ts";
import { openPopupWindow } from "./popup";
import { getFromStorage, saveToStorage } from "./storage";

export interface ConnectRequest {
  origin: string;
}

export async function handleConnectRequest(
  requestId: string,
  request: ConnectRequest,
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
  if (connectedWebsites[origin]) {
    port.postMessage({
      requestId,
      result: [connectedWebsites[origin]],
    });

    return;
  }

  // Open popup for new connection request
  await openPopupWindow(`/connect?requestId=${requestId}&origin=${encodeURIComponent(origin)}`);
}

type ConnectionResponseMessage = {
  requestId: string;
  smartAccountAddress?: string;
  origin?: string;
};

type ConnectedWebsites = Record<string, boolean>;

export function handleConnectionResponse(
  message: ConnectionResponseMessage,
  port: chrome.runtime.Port,
) {
  const { requestId, smartAccountAddress, origin } = message;

  if (smartAccountAddress && origin) {
    // Store connection
    getFromStorage("connectedWebsites").then((connectedWebsites) => {
      const updatedWebsites = { ...connectedWebsites, [origin]: smartAccountAddress };
      saveToStorage<ConnectedWebsites>("connectedWebsites", updatedWebsites);
    });

    // Return wallet address to the content script
    port.postMessage({
      requestId,
      result: [smartAccountAddress],
    });
  } else {
    // Handle user rejection with standardized error
    port.postMessage({
      requestId,
      error: createError(ERROR_CODES.USER_REJECTED, ERROR_MESSAGES.USER_REJECTED),
    });
  }
}
