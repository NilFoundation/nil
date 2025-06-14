import { COLORS } from "@nilfoundation/ui-kit";
import type { AreaSeriesPartialOptions, AutoscaleInfoProvider } from "lightweight-charts";

const autoscaleInfoProvider: AutoscaleInfoProvider = (original) => {
  const res = original();

  if (!res || !res.priceRange) {
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
  priceScaleId: "left",
  priceLineVisible: false,
  lastValueVisible: false,
  topColor: COLORS.gray200,
  lineWidth: 1,
  lineColor: COLORS.gray100,
  bottomColor: "transparent",
  crosshairMarkerBackgroundColor: COLORS.gray900,
  crosshairMarkerBorderColor: COLORS.gray100,
  crosshairMarkerBorderWidth: 1,
  autoscaleInfoProvider,
};
