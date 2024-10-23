import type { AxiosInstance } from "axios";
import { config } from "../config.ts";

interface RawTransaction {
  "@type": string;
  data: string;
  fee: string;
  in_msg?: {
    "@type": string;
    body_hash: string;
    created_lt: string;
    destination: string;
    fwd_fee: string;
    ihr_fee: string;
    message: string;
    msg_data: {
      "@type": string;
      body: string;
      init_state: string;
    };
    source: string;
    value: string;
  };
  other_fee: string;
  out_msgs: {
    "@type": string;
    body_hash: string;
    created_lt: string;
    destination: string;
    fwd_fee: string;
    ihr_fee: string;
    message: string;
    msg_data: {
      "@type": string;
      body: string;
      init_state: string;
    };
    source: string;
    value: string;
  }[];
  storage_fee: string;
  transaction_id: {
    "@type": string;
    hash: string;
    lt: string;
  };
  utime: number;
}

export const fetchTransactions = async (client: AxiosInstance, address: string, page = 1) => {
  const offset = (page - 1) * 10;
  return client
    .post<{ result: RawTransaction[] }>(config.RPC_URL, {
      jsonrpc: "2.0",
      id: 1,
      method: "getTransactions",
      params: { address: address, limit: 10, hash: "", offset },
    })
    .then((res) => {
      return res.data.result;
    });
};
