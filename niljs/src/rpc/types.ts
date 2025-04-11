interface JsonRpcRequest {
  jsonrpc: "2.0";
  readonly method: string;
  readonly params: readonly unknown[] | object;
  readonly id: number;
}

interface JsonRpcResponse<T> {
  jsonrpc: "2.0";
  id: number;
  result?: T;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
}

export type { JsonRpcRequest, JsonRpcResponse };
