import { COLORS, LabelSmall } from "@nilfoundation/ui-kit";
import { expandProperty } from "inline-style-expand-shorthand";
import type { BusinessDay } from "lightweight-charts";
import {
  Chart,
  CustomSeries,
  PriceScale,
  TimeScale,
  TimeScaleFitContentTrigger,
} from "lightweight-charts-react-components";
import { useStyletron } from "styletron-react";
import type { GroupedBarsData } from "./SeriesPlugin/GroupedBarsData";
import { GroupedBarsSeries } from "./SeriesPlugin/plugin";
import { chartDefaultOptions } from "./chartDefaultOptions";
import { svgInlined } from "./pattern";

interface TotalTxChartProps {
  shardsGasPriceData: GroupedBarsData[];
  shardsTxData: GroupedBarsData[];
}

const timeFormatter = (t: BusinessDay) => `#${t.year}`;

export const TotalTxChart = ({ shardsGasPriceData, shardsTxData }: TotalTxChartProps) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
        alignItems: "center",
        flexGrow: "1",
      })}
    >
      <Chart
        containerProps={{ style: { flexGrow: "1", width: "100%" } }}
        options={chartDefaultOptions}
      >
        <CustomSeries
          plugin={new GroupedBarsSeries()}
          data={shardsGasPriceData}
          options={{
            priceLineVisible: false,
            priceScaleId: "right",
            lastValueVisible: false,
          }}
        />
        <CustomSeries
          plugin={new GroupedBarsSeries()}
          data={shardsTxData}
          options={{
            priceLineVisible: false,
            priceScaleId: "left",
            lastValueVisible: false,
          }}
        />
        <PriceScale
          id="right"
          options={{
            scaleMargins: {
              top: 0.5,
              bottom: 0,
            },
            entireTextOnly: true,
            ticksVisible: true,
          }}
        />
        <PriceScale
          id="left"
          options={{
            scaleMargins: {
              bottom: 0,
              top: 0.1,
            },
            entireTextOnly: true,
            ticksVisible: true,
          }}
        />
        <TimeScale
          options={{
            tickMarkFormatter: timeFormatter,
          }}
        >
          <TimeScaleFitContentTrigger deps={[]} />
        </TimeScale>
      </Chart>
      <div
        className={css({
          display: "flex",
          gap: "16px",
          width: "100%",
          marginTop: "10px",
        })}
      >
        <div
          className={css({
            display: "flex",
            gap: "4px",
            alignItems: "center",
          })}
        >
          <div
            className={css({
              width: "12px",
              height: "12px",
              backgroundColor: COLORS.gray50,
              ...expandProperty("borderRadius", "4px"),
            })}
          />
          <LabelSmall color={COLORS.gray400}>Transactions on a shard</LabelSmall>
        </div>
        <div
          className={css({
            display: "flex",
            gap: "4px",
            alignItems: "center",
          })}
        >
          <div
            className={css({
              width: "12px",
              height: "12px",
              backgroundImage: `url(${svgInlined})`,
              backgroundRepeat: "repeat",
              backgroundSize: "6px 6px",
              ...expandProperty("borderRadius", "4px"),
            })}
          />
          <LabelSmall color={COLORS.gray400}>Gas price in Gwei</LabelSmall>
        </div>
      </div>
    </div>
  );
};
