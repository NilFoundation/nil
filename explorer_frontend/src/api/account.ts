import { client } from "./client";

export const fetchAccountState = async (address: string) => {
  const res = await client.account.state.query(address);
  return res;
};
