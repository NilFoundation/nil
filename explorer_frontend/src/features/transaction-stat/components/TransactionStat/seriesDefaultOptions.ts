import { COLORS } from "@nilfoundation/ui-kit";
import type { AreaSeriesPartialOptions, AutoscaleInfoProvider } from "lightweight-charts";

const autoscaleInfoProvider: AutoscaleInfoProvider = (original) => {
  const res = original();

  if (!res) {
    return null;
  }

  return {
    priceRange: {
      minValue: 0,
      maxValue: res.priceRange.maxValue,
    },
    margins: {
      above: 10,
      below: 10,
    },
  };
};

export const seriesDefaultOptions: AreaSeriesPartialOptions = {
  priceFormat: {
    type: "price",
    precision: 2,
    minMove: 0.01,
  },
  priceScaleId: "left",
  priceLineVisible: false,
  lastValueVisible: false,
  topColor: COLORS.blue400,
  autoscaleInfoProvider,
};
