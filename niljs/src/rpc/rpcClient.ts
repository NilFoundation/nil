interface RequestArguments {
  readonly method: string;
  readonly params?: readonly unknown[] | object;
  readonly id?: number;
  jsonrpc?: "2.0";
}

class RPCClient {
  private endpoint: string;
  private headers: Record<string, string>;
  private signal?: AbortSignal;

  constructor(endpoint: string, { signal, headers = {} }: { signal?: AbortSignal; headers?: Record<string, string> } = {}) {
    this.endpoint = endpoint;
    this.headers = {
      "Client-Version": `niljs/${version}`,
      ...headers,
    };
    this.signal = signal;
  }

  async request({ method, params }: RequestArguments): Promise<any> {
    const response = await fetch(this.endpoint, {
      method: "POST",
      headers: this.headers,
      body: JSON.stringify({ method, params }),
      signal: this.signal,
    });

    if (!response.ok) {
      throw new Error(`Error: ${response.statusText}`);
    }

    return response.json();
  }
}

export { RPCClient};
