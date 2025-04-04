import { client } from "./client";

export const setProject = async ({ code, script }: { code: string, script: string | null }) => {
  const { hash } = await client.code.set.mutate({ code, script: script ?? undefined });

  return hash;
};

export const fetchProject = async (hash: string) => {
  const res = await client.code.get.query(hash);

  return {
    code: res.code,
    script: res.script ?? null,
  };
};
