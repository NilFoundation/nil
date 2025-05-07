import { StyledChart as ChartWrapper, HeadingMedium, Spinner } from "@nilfoundation/ui-kit";
import { useStore, useUnit } from "effector-react";
import {
  type AreaData,
  type ISeriesApi,
  type LineData,
  MismatchDirection,
  type MouseEventParams,
  type Time,
} from "lightweight-charts";
import {
  AreaSeries,
  PriceScale,
  type SeriesApiRef,
  TimeScale,
} from "lightweight-charts-react-components";
import { useCallback, useMemo, useRef, useState } from "react";
import { useStyletron } from "styletron-react";
import { $latestBlocks } from "../../../latest-blocks/models/model";
import { formatNumber, useMobile } from "../../../shared";
import { $timeInterval, $transactionsStat, fetchTransactionsStatFx } from "../../models/model";
import { Legend } from "./Legend";
import { TimeIntervalToggle } from "./TimeIntervalToggle";
import { Tooltip } from "./Tooltip";
import { getChartDefaultOptions } from "./chartDefaultOptions";
import { seriesDefaultOptions } from "./seriesDefaultOptions";
import { styles as s } from "./styles";
import { useGetLogicalRange } from "./useGetLogicalRange";
import { useTooltip } from "./useTooltip";

const refineLegendValue = (value: string) => {
  if (!Number.isNaN(Number(value))) {
    return formatNumber(Number(value));
  }
  return value;
};

export const Chart = () => {
  const [isMobile] = useMobile();
  const [param, setParam] = useState<MouseEventParams>();
  const [transactionsStat, pending, latestBlocks] = useUnit([
    $transactionsStat,
    fetchTransactionsStatFx.pending,
    $latestBlocks,
  ]);
  const timeInterval = useStore($timeInterval);
  const now = Math.round(Date.now() / (1000 * 60)) * 1000 * 60;
  let lastBlock = 0;
  if (transactionsStat.length > 0) {
    lastBlock = transactionsStat[0].earliest_block;
  }
  if (latestBlocks.length > 0) {
    lastBlock = Number(latestBlocks[0].id);
  }

  const containerRef = useRef<HTMLDivElement>(null);
  const [css] = useStyletron();
  const series = useRef<SeriesApiRef<"Area">>(null);

  const getLastBarValue = useCallback((api: ISeriesApi<"Area">) => {
    const last = api.dataByIndex(
      Number.POSITIVE_INFINITY,
      MismatchDirection.NearestLeft,
    ) as LineData;
    return last?.value.toFixed() ?? "-";
  }, []);

  const mappedData = useMemo(
    () =>
      transactionsStat
        .map((item) => ({
          time: (now - ((lastBlock - item.earliest_block) * 60000) / 29) as Time,
          value: item.value,
        }))
        .reverse(),
    [transactionsStat, now, lastBlock],
  );

  // biome-ignore lint/correctness/useExhaustiveDependencies: <explanation>
  const legend = useMemo(() => {
    const seriesApi = series.current?.api();

    if (!seriesApi) {
      return "-";
    }

    if (!param || !param.time) {
      return refineLegendValue(getLastBarValue(seriesApi));
    }

    const { value } = param.seriesData.get(seriesApi) as AreaData;
    return refineLegendValue(value?.toFixed());
  }, [param, mappedData, getLastBarValue]);

  const handleCrosshairMove = useCallback((param: MouseEventParams) => {
    setParam(param);
  }, []);

  const range = useGetLogicalRange(mappedData.length);

  const tooltipWidth = 140;
  const tooltipHeight = 100;
  const tooltipMargin = 10;
  const { isOpen, position } = useTooltip(
    param,
    containerRef.current,
    isMobile,
    tooltipMargin,
    tooltipWidth,
    tooltipHeight,
  );

  return (
    <div
      className={css({
        height: "100%",
        width: "100%",
        position: "relative",
        display: "flex",
        flexDirection: "column",
      })}
    >
      <TimeIntervalToggle timeInterval={timeInterval} />
      <Legend value={legend} />
      <div className={css(s.chartContainer)} ref={containerRef} data-testid="transaction-chart">
        {transactionsStat.length === 0 && pending ? (
          <Spinner />
        ) : transactionsStat.length > 0 ? (
          <ChartWrapper
            onCrosshairMove={handleCrosshairMove}
            containerProps={{ className: css(s.chart) }}
            options={getChartDefaultOptions(timeInterval)}
          >
            <AreaSeries data={mappedData} reactive options={seriesDefaultOptions} ref={series}>
              <PriceScale id="left" options={{}} />
            </AreaSeries>
            <TimeScale visibleLogicalRange={range} />
          </ChartWrapper>
        ) : (
          <HeadingMedium>No data to display</HeadingMedium>
        )}
      </div>
      {!isMobile && (
        <Tooltip
          data={{
            time: param?.time,
            tps: legend,
          }}
          isOpen={isOpen}
          position={position}
          width={tooltipWidth}
        />
      )}
    </div>
  );
};
