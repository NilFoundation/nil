import { COLORS } from "@nilfoundation/ui-kit";
import type { UTCTimestamp } from "lightweight-charts";
import type { ChartOptions, DeepPartial } from "lightweight-charts";
import { formatToK } from "../../../shared";
import { formatUTCTimestamp } from "../../../shared/utils/formatUTCTimestamp";
import { TimeInterval } from "../../types/TimeInterval";

const getTimeFormatter = (timeInterval: TimeInterval) => (t: UTCTimestamp) =>
  formatUTCTimestamp(t, timeInterval === TimeInterval.OneDay ? "HH:mm" : "DD.MM");
const priceFormatter = (p: number) => formatToK(p);

export const getChartDefaultOptions = (timeInterval: TimeInterval): DeepPartial<ChartOptions> => ({
  autoSize: true,
  layout: {
    background: {
      color: "transparent",
    },
    textColor: COLORS.gray400,
    attributionLogo: false,
  },
  localization: {
    timeFormatter: getTimeFormatter(timeInterval),
    priceFormatter: priceFormatter,
  },
  timeScale: {
    fixRightEdge: true,
    fixLeftEdge: true,
    tickMarkFormatter: getTimeFormatter(timeInterval),
  },
  crosshair: {
    vertLine: {
      color: COLORS.gray400,
      width: 1,
      style: 1,
      labelVisible: true,
      visible: true,
    },
    horzLine: {
      visible: false,
      labelVisible: false,
    },
    mode: 0,
  },
  leftPriceScale: {
    scaleMargins: {
      top: 0.2,
      bottom: 0,
    },
    visible: true,
    borderVisible: false,
  },
  rightPriceScale: {
    visible: false,
    borderVisible: false,
  },
});
