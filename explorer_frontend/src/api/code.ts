import { client } from "./client";

export const setCodeSnippet = async ({ code, script }: { code: string, script: string | null }) => {
  const { hash } = await client.code.set.mutate({ code, script: script ?? undefined });

  return hash;
};

export const fetchCodeSnippet = async (hash: string) => {
  const res = await client.code.get.query(hash);

  return {
    code: res.code,
    script: res.script,
  };
};
