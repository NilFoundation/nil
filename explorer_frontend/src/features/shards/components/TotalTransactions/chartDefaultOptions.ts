import { COLORS } from "@nilfoundation/ui-kit";
import type { ChartProps } from "lightweight-charts-react-components";
import { formatToK } from "../../../shared/utils/formatNumber";

const totalTxCountFormatter = (p: number) => formatToK(p);

export const chartDefaultOptions: ChartProps["options"] = {
  autoSize: true,
  handleScroll: true,
  handleScale: false,
  layout: {
    background: {
      color: "transparent",
    },
    textColor: COLORS.gray400,
    attributionLogo: false,
  },
  localization: {
    locale: "en-US",
    priceFormatter: totalTxCountFormatter,
  },
  timeScale: {
    fixRightEdge: true,
    fixLeftEdge: true,
  },
  crosshair: {
    vertLine: {
      labelVisible: false,
      visible: false,
    },
    horzLine: {
      visible: false,
      labelVisible: false,
    },
    mode: 0,
  },
  leftPriceScale: {
    visible: true,
    borderVisible: false,
  },
  rightPriceScale: {
    visible: true,
    borderVisible: false,
  },
  grid: {
    vertLines: {
      visible: false,
    },
    horzLines: {
      visible: false,
    },
  },
};
