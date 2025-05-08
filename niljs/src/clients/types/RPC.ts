import type { JSONRPCID } from "json-rpc-2.0";

// biome-ignore lint/suspicious/noExplicitAny: <explanation>
type JSONRPCParams = Array<any> | object;

/**
 * Represents a JSON-RPC request arguments.
 */
type JSONRPCRequestArguments = {
  method: string;
  params?: JSONRPCParams;
  id?: JSONRPCID;
};

export type { JSONRPCRequestArguments };
