export const getShardIdAndHeight = (query: string) => {
  const [shardId, height] = query.split(":");
  return { shardId: +shardId, height: +height };
};

export const shardIdAndHeightRegExp = /(\d+)\s*:\s*(\d+)/;
