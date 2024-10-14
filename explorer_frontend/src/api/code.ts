import { client } from "./client";

export const setCodeSnippet = async (code: string) => {
  const { hash } = await client.code.set.mutate(code);

  return hash;
};

export const fetchCodeSnippet = async (hash: string) => {
  const res = await client.code.get.query(hash);

  return res.code;
};
