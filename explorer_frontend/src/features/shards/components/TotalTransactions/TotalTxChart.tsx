import { Chart, CustomSeries } from "lightweight-charts-react-components";
import { GroupedBarsSeries } from "./SeriesPlugin/plugin";
import { useStyletron } from "styletron-react";
import { chartDefaultOptions } from "./chartDefaultOptions";
import { COLORS, LabelSmall } from "@nilfoundation/ui-kit";
import { expandProperty } from "inline-style-expand-shorthand";

export const TotalTxChart = () => {
  const [css] = useStyletron();
  const data = [
    {
      time: "2021-01-01",
      customValues: {
        values: [1, 2, 3],
      },
    },
    {
      time: "2021-01-02",
      customValues: {
        values: [2, 3, 4],
      },
    },
    {
      time: "2021-01-03",
      customValues: {
        values: [3, 4, 5],
      },
    },
  ];

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
      <Chart containerProps={{ style: { flexGrow: "1" } }} options={chartDefaultOptions}>
        <CustomSeries plugin={new GroupedBarsSeries()} data={data} />
      </Chart>
      <div
        className={css({
          display: "flex",
          gap: "16px",
          width: "100%",
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
              backgroundColor: COLORS.gray50,
              ...expandProperty("borderRadius", "4px"),
            })}
          />
          <LabelSmall color={COLORS.gray400}>Gas price in Gwei</LabelSmall>
        </div>
      </div>
    </div>
  );
};
