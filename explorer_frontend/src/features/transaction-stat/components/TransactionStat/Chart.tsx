import {
  Spinner,
  Chart as ChartWrapper,
  AreaSeries,
  TimeScale,
  HeadingMedium,
  type SeriesApiRef,
} from "@nilfoundation/ui-kit";
import { useCallback, useMemo, useRef, useState } from "react";
import { getChartDefaultOptions } from "./chartDefaultOptions";
import { Legend } from "./Legend";
import { styles as s } from "./styles";
import { useStyletron } from "styletron-react";
import {
  type LineData,
  MismatchDirection,
  type MouseEventParams,
  type Time,
  type AreaData,
  type ISeriesApi,
} from "lightweight-charts";
import { TimeIntervalToggle } from "./TimeIntervalToggle";
import { seriesDefaultOptions } from "./seriesDefaultOptions";
import { useGetLogicalRange } from "./useGetLogicalRange";
import { Tooltip } from "./Tooltip";
import { formatNumber, useMobile } from "../../../shared";
import { useTooltip } from "./useTooltip";
import { useStore, useUnit } from "effector-react";
import { $timeInterval, $transactionsStat, fetchTransactionsStatFx } from "../../models/model";
import { $latestBlocks } from "../../../latest-blocks/models/model";

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
  const tooltipMargin = 10;
  const { isOpen, position } = useTooltip(
    param,
    containerRef.current,
    isMobile,
    tooltipMargin,
    tooltipWidth,
  );

  return (
    <>
      <Legend value={legend} />
      <TimeIntervalToggle timeInterval={timeInterval} />
      <div className={css(s.chartContainer)} ref={containerRef}>
        {transactionsStat.length === 0 && pending ? (
          <Spinner animation />
        ) : transactionsStat.length > 0 ? (
          <ChartWrapper
            onCrosshairMove={handleCrosshairMove}
            className={css(s.chart)}
            {...getChartDefaultOptions(timeInterval)}
          >
            <AreaSeries data={mappedData} reactive options={seriesDefaultOptions} ref={series} />
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
    </>
  );
};
