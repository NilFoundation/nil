import type { Abi } from "abitype";

export type App = {
  name: string;
  bytecode: `0x${string}`;
  sourcecode: string;
  abi: Abi;
};

export type Tutorial = {
  id: string;
  text: string;
  contracts: string;
  stage: number;
};
