import { ACTIONS } from "../../background/constants.ts";
import { ERROR_CODES, ERROR_MESSAGES } from "../../features/utils/errors.ts";
import type { WindowNilRequest } from "../types.ts";
import { ExtensionMethods } from "./methods.ts";

export class ExtensionMethodHandler {
  private port: chrome.runtime.Port | null = null;
  private pendingRequests = new Set<string>();

  constructor() {
    this.connectPort();
  }

  // Connect to the background script if not already connected
  private connectPort(): void {
    // Skip if already connected
    if (this.port) return;

    try {
      this.port = chrome.runtime.connect({ name: "extension-handler" });

      // Listen for responses
      this.port.onMessage.addListener((message) => {
        const { requestId } = message;

        if (requestId) {
          this.pendingRequests.delete(requestId);
        }

        window.postMessage(message, "*");
      });

      // Handle disconnection
      this.port.onDisconnect.addListener(() => {
        this.port = null;

        // Reconnect only if there are pending requests
        if (this.pendingRequests.size > 0) {
          console.warn(`Reconnecting due to ${this.pendingRequests.size} pending requests...`);
          this.connectPort();
        }
      });
    } catch (error) {
      console.error("Failed to connect port:", error);
      this.port = null;
    }
  }

  // Send a request, reconnecting if necessary
  private request(request: WindowNilRequest, action: string): void {
    if (!this.port) {
      this.connectPort();
    }

    if (this.port) {
      try {
        this.pendingRequests.add(request.requestId);
        this.port.postMessage({ action, request });
      } catch (error) {
        window.postMessage(
          {
            requestId: request.requestId,
            error: {
              code: ERROR_CODES.INVALID_PARAMS,
              message: "Failed to send request. Please check your inputs and avoid using BigInt.",
            },
          },
          "*",
        );

        // Reset port if postMessage fails
        this.port?.disconnect();
        this.port = null;
      }
    }
  }

  // Handle incoming requests and forward them to the background
  async handleRequest(request: WindowNilRequest): Promise<void> {
    switch (request.method) {
      case ExtensionMethods.eth_sendTransaction:
        this.request(request, ACTIONS.PROCESS_REQUEST);
        break;

      case ExtensionMethods.eth_requestAccounts:
        this.request(request, ACTIONS.CONNECT);
        break;

      default:
        window.postMessage(
          {
            requestId: request.requestId,
            error: {
              code: ERROR_CODES.UNSUPPORTED_METHOD,
              message: ERROR_MESSAGES.UNSUPPORTED_METHOD(request.method),
            },
          },
          "*",
        );
        break;
    }
  }
}
