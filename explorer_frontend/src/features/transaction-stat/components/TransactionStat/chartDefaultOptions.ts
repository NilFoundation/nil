import type { UTCTimestamp } from "lightweight-charts";
import type { ChartOptions, DeepPartial } from "lightweight-charts";
import { formatUTCTimestamp } from "../../../shared/utils/formatUTCTimestamp";
import { formatNumber } from "../../../shared";
import { COLORS } from "@nilfoundation/ui-kit";
import { TimeInterval } from "../../types/TimeInterval";

const getTimeFormatter = (timeInterval: TimeInterval) => (t: UTCTimestamp) =>
  formatUTCTimestamp(t, timeInterval === TimeInterval.OneDay ? "YYYY DD.MM" : "YYYY DD.MM HH:mm");
const priceFormatter = (p: number) => formatNumber(p);

export const getChartDefaultOptions = (timeInterval: TimeInterval): DeepPartial<ChartOptions> => ({
  layout: {
    background: {
      color: "transparent",
    },
    textColor: COLORS.gray400,
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
      width: 2,
      style: 0,
      labelVisible: true,
      visible: true,
    },
    horzLine: {
      color: COLORS.gray400,
      width: 1,
      style: 0,
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
});
