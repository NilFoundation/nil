import { COLORS } from "@nilfoundation/ui-kit";

import { customSeriesDefaultOptions } from "lightweight-charts";
import type { CustomSeriesOptions } from "lightweight-charts";

export interface GroupedBarsSeriesOptions extends CustomSeriesOptions {
  colors: readonly string[];
}

export const defaultOptions: GroupedBarsSeriesOptions = {
  ...customSeriesDefaultOptions,
  colors: [COLORS.gray50, COLORS.gray200],
} as const;
