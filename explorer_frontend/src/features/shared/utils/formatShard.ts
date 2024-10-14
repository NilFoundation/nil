export const formatShard = (shard: string, id: string) => {
  return `${`${shard}`.padStart(2, "0")}:${`${id}`.padStart(6, "0")}`;
};
