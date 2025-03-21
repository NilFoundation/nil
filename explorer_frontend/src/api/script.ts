import { client } from "./client";

export const setScript = async (script: string) => {
  const { hash } = await client.script.set.mutate(script);

  return hash;
};

export const fetchScript = async (script: string) => {
  const res = await client.script.get.query(script);

  return res.script;
};
