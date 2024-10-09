export interface RawTransaction {
  "@type": string;
  data: string;
  fee: string;
  in_msg: {
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

export type SessionInfo = {
  address: string;
};
