export type Flag = {
  isExternal: boolean;
  isDeploy: boolean;
  isRefund: boolean;
  isBounce: boolean;
};

export const processFlag = (flag: number) => {
  return {
    isExternal: (flag & 1) === 1,
    isDeploy: (flag & 2) === 2,
    isRefund: (flag & 4) === 4,
    isBounce: (flag & 8) === 8,
  };
};
