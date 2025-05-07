import type { Range } from "lightweight-charts";
import { useMemo } from "react";
import { useMobile } from "../../../shared/hooks/useMobile";

export const useGetLogicalRange = (dataLength: number): Range<number> => {
  const [isMobile] = useMobile();

  const range = useMemo(() => {
    if (dataLength === 0) {
      return { from: 0, to: 0 };
    }

    const to = dataLength - 1;

    if (!isMobile) {
      return { from: 0, to };
    }

    const from = dataLength - 5 > 0 ? dataLength - 5 : 0;

    return { from, to };
  }, [dataLength, isMobile]);

  return range;
};
