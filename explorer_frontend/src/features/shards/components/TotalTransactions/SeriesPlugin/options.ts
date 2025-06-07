import { COLORS } from "@nilfoundation/ui-kit";
import { customSeriesDefaultOptions } from "lightweight-charts";
import type { CustomSeriesOptions } from "lightweight-charts";
import { getSvgPattern } from "../pattern";

type GetPattern = (ctx: CanvasRenderingContext2D) => CanvasPattern | null;

export type Colors = Array<string | GetPattern>;

export interface GroupedBarsSeriesOptions extends CustomSeriesOptions {
  colors: Colors;
}

export const defaultOptions: GroupedBarsSeriesOptions = {
  ...customSeriesDefaultOptions,
  colors: [COLORS.gray50, (ctx) => getSvgPattern(ctx)],
} as const;
