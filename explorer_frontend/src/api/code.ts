import { client } from "./client";

export const setProject = async (project: Record<string, string>) => {
  const { hash } = await client.code.set.mutate(project);
  return hash;
};

export const fetchProject = async (hash: string): Promise<Record<string, string>> => {
  const res = await client.code.get.query(hash);
  return res;
};
