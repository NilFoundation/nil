import { rpcErrors, serializeError } from "@metamask/rpc-errors";
import EventEmitter from "eventemitter3";
import { v4 as uuidv4 } from "uuid";
import { ZodError } from "zod";
import {
  addWindowMessageListener,
  removeWindowMessageListener,
} from "../background/messagePassing/messageUtils.ts";
import { type BaseEthereumRequest, BaseNilRequestSchema } from "./WindowNilRequestTypes.ts";
import {
  type EthersSendCallback,
  type ExtensionResponse,
  type RequestInput,
  isValidExtensionResponse,
} from "./types.ts";

const messages = {
  errors: {
    disconnected: (): string => "=nil; Wallet: Disconnected from chain. Attempting to connect",
    invalidRequestArgs: (): string => "=nil; Wallet: Expected a single, non-array, object argument",
    invalidRequestGeneric: (): string =>
      "=nil; Wallet: Please check the input passed to the request method",
  },
};

export class WindowNilProxy extends EventEmitter {
  /**
   * Boolean indicating that the provider is NIL Wallet.
   */
  isNilWallet = true;

  /**
   * Pending requests are stored as promises that resolve or reject based on the response from the content script.
   */
  pendingRequests: {
    [key: string]: {
      resolve: (value: unknown) => void;
      reject: (error: unknown) => void;
    };
  };

  constructor() {
    super();
    this.pendingRequests = {};
  }

  // Deprecated EIP-1193 method
  send = (
    methodOrRequest: string | BaseEthereumRequest,
    paramsOrCallback: Array<unknown> | EthersSendCallback,
  ): Promise<unknown> | undefined => {
    throw new Error("Deprecated method not supported by Nil Wallet. Use `request` instead");
  };

  // Deprecated EIP-1193 method still in use by some DApps
  sendAsync = (
    request: RequestInput,
    callback: (error: unknown, response: unknown) => void,
  ): Promise<unknown> | undefined => {
    throw new Error("Deprecated method not supported by Nil Wallet. Use `request` instead.");
  };

  request = async (args: RequestInput): Promise<unknown> => {
    return new Promise((resolve, reject) => {
      try {
        const nilRequest = BaseNilRequestSchema.parse(args);

        // Generate a unique ID for this request and store the promise callbacks
        const requestId = uuidv4();
        this.pendingRequests[requestId] = { resolve, reject };

        const responseListener = addWindowMessageListener<ExtensionResponse>(
          isValidExtensionResponse,
          (response) => {
            if (response.requestId === requestId) {
              this.handleResponse(response);
              removeWindowMessageListener(responseListener);
            }
          },
        );

        window.postMessage({
          ...nilRequest,
          requestId,
        });
      } catch (error) {
        // Based on the zod error, we can determine the type of error and reject accordingly
        if (error instanceof ZodError) {
          return reject(
            serializeError(
              rpcErrors.invalidRequest({
                message: messages.errors.invalidRequestArgs(),
                data: {
                  ...args,
                  cause: undefined,
                },
              }),
            ),
          );
        }

        return reject(
          serializeError(
            rpcErrors.invalidRequest({
              message: messages.errors.invalidRequestGeneric(),
              data: {
                ...args,
                cause: undefined,
              },
            }),
          ),
        );
      }
    });
  };

  private handleResponse(response: ExtensionResponse) {
    const { requestId, result, error } = response;
    const promise = this.pendingRequests[requestId];

    if (!promise) {
      return;
    }

    if (error) {
      promise.reject(error);
      delete this.pendingRequests[requestId];
    }

    promise.resolve(result);

    // Clean up after handling the response
    delete this.pendingRequests[requestId];
  }
}
