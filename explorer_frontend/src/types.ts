import type { Abi } from "abitype";

export interface AbiFile {
  "ABI version": number;
  version: string;
  header: string[];
  functions: FunctionInfo[];
  fields: FieldInfo[];
  events: unknown[];
}

export type DeployedApp = {
  abi: Abi;
  address: string;
  code: string;
  state?: Record<string, string>;
};

export interface FunctionInfo {
  name: string;
  inputs: InputParameter[];
  outputs: OutputParameter[];
  id: string;
}

export interface InputParameter {
  name: string;
  type: string;
}

export interface OutputParameter {
  name: string;
  type: string;
}

export interface FieldInfo {
  name: string;
  type: string;
}

export interface RawTransaction {
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

export type ContractState = {
  decoded: null | Record<string, string | boolean>;
  balance: string;
  state: string;
};

export type App = {
  name: string;
  bytecode: `0x${string}`;
  sourcecode: string;
  abi: Abi;
};
