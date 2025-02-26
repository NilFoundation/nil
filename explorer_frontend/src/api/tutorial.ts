import { client } from "./client";

export const fetchTutorial = async (stage: string) => {
  const stageForQuery = Number(stage);
  const res = await client.tutorial.get.query(stageForQuery);

  return { ...res, stage: Number(res.stage) };
};

export const fetchAllTutorials = async () => {
  const res = await client.tutorial.getAll.query();

  return res;
};
