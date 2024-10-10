import axios from "axios";
import { TonClient } from "ton";
import type { AxiosInstance } from "axios";
import { config } from "../config.ts";

export const getToken = async (): Promise<string> => {
  const res = await axios.post<{ jwt: string }>(`${config.DB_URL}/_db/_system/_open/auth`, {
    username: config.DB_USER,
    password: config.DB_PASSWORD,
  });

  return res.data.jwt;
};

export const fetchClients = async (): Promise<[TonClient, AxiosInstance]> => {
  return [
    new TonClient({
      endpoint: config.RPC_URL,
    }),
    axios.create({
      baseURL: config.DB_URL,
      family: 4,
    }),
  ];
};
