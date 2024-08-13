interface Eip1193Provider {
  request(request: {
    method: string;
    // biome-ignore lint/suspicious/noExplicitAny: <explanation>
    params?: Array<any> | Record<string, any>;
    // biome-ignore lint/suspicious/noExplicitAny: <explanation>
  }): Promise<any>;
}

declare global {
  interface Window {
    ethereum?: Eip1193Provider;
  }
}
