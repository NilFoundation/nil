export type CallParams = Record<
  string,
  Record<
    string,
    {
      type: string;
      value:
        | string
        | boolean
        | {
            type: string;
            value: string | boolean;
          }[];
    }
  >
>;
