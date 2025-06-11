import { COLORS, HeadingMedium, LabelLarge, ParagraphXSmall, Spinner } from "@nilfoundation/ui-kit";
import { ParagraphLarge } from "baseui/typography";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { useMemo } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { useStyletron } from "styletron-react";
import { formatEther } from "viem";
import { getMobileStyles } from "../../../../styleHelpers";
import { InfoContainer } from "../../../shared";
import { formatNumber } from "../../../shared/utils/formatNumber";
import {
  $bestShard,
  $busiestShard,
  $shards,
  $shardsGasPriceMap,
  $totalTxOnShards,
  fetchShardsFx,
  fetchShrdsGasPriceFx,
} from "../../models/model";
import { TotalTxChart } from "./TotalTxChart";

const ErrorView = () => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        height: "100%",
        width: "100%",
      })}
    >
      <ParagraphLarge
        className={css({
          textAlign: "center",
        })}
      >
        An error occurred while displaying the total transactions chart. Please try again later.
      </ParagraphLarge>
    </div>
  );
};

export const TotalTransactions = () => {
  const [
    shards,
    totalTxCount,
    busiestShard,
    bestShard,
    fetchingShardsGasPrice,
    fetchingShardsStat,
    shardsGasPriceMap,
  ] = useUnit([
    $shards,
    $totalTxOnShards,
    $busiestShard,
    $bestShard,
    fetchShrdsGasPriceFx.pending,
    fetchShardsFx.pending,
    $shardsGasPriceMap,
  ]);
  const [css] = useStyletron();

  const shardsCount = Object.keys(shards).length;
  const busiestShardId = busiestShard?.[0];
  const bestShardId = bestShard?.[0];
  const busiestShardGasPrice = busiestShard?.[1] ? `${formatEther(busiestShard?.[1])} ETH` : "-";
  const bestShardGasPrice = bestShard?.[1] ? `${formatEther(bestShard?.[1])} ETH` : "-";

  const shardsTxData = useMemo(
    () =>
      shards
        .map((shard) => ({
          time: {
            year: shard.shard_id,
            month: 1,
            day: 1,
          },
          customValues: {
            values: [shard.tx_count, 0],
          },
        }))
        .sort((a, b) => a.time.year - b.time.year),
    [shards],
  );

  const shardsGasPriceData = useMemo(
    () =>
      Object.entries(shardsGasPriceMap)
        .map(([shardId, gasPrice]) => ({
          time: {
            year: Number(shardId),
            month: 1,
            day: 1,
          },
          customValues: {
            values: [0, Number(formatEther(gasPrice, "gwei")).toFixed(1)],
          },
        }))
        .sort((a, b) => a.time.year - b.time.year),
    [shardsGasPriceMap],
  );

  console.log(shardsGasPriceData);

  if ((fetchingShardsGasPrice || fetchingShardsStat) && shardsTxData.length === 0) {
    return (
      <InfoContainer title="Total transactions across shards">
        <div
          className={css({
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            height: "100%",
            width: "100%",
          })}
        >
          <Spinner />
        </div>
      </InfoContainer>
    );
  }

  if (shardsCount === 1 && shards[0].shard_id === 0) {
    return (
      <InfoContainer title="Total transactions across shards">
        <ParagraphLarge>Only main shard exists</ParagraphLarge>
      </InfoContainer>
    );
  }

  return (
    <InfoContainer title={`Total transactions across ${shardsCount} shards`}>
      <ErrorBoundary fallback={<ErrorView />}>
        <div
          className={css({
            display: "flex",
            gap: "32px",
            flexDirection: "column",
            height: "100%",
          })}
        >
          <LabelLarge
            className={css({
              fontSize: "24px",
              lineHeight: "32px",
              marginTop: "8px",
            })}
          >
            {formatNumber(totalTxCount)}
          </LabelLarge>
          <div
            className={css({
              display: "flex",
              gap: "8px",
              justifyContent: "center",
              ...getMobileStyles({
                flexDirection: "column",
              }),
            })}
          >
            <div
              className={css({
                flexBasis: "50%",
                ...expandProperty("borderRadius", "16px"),
                ...expandProperty("padding", "16px"),
                backgroundColor: COLORS.gray800,
              })}
            >
              <HeadingMedium>{`Busiest shard: ${busiestShardId ? `#${busiestShardId}` : "-"}`}</HeadingMedium>
              <ParagraphXSmall
                color={COLORS.gray200}
                marginTop="8px"
              >{`Gas price: ${busiestShardGasPrice}`}</ParagraphXSmall>
            </div>
            <div
              className={css({
                flexBasis: "50%",
                ...expandProperty("borderRadius", "16px"),
                ...expandProperty("padding", "16px"),
                ...expandProperty("border", `1px solid ${COLORS.green200}`),
                backgroundColor: COLORS.green800,
              })}
            >
              <HeadingMedium>{`Best deployment shard: ${bestShardId ? `#${bestShardId}` : "-"}`}</HeadingMedium>
              <ParagraphXSmall
                color={COLORS.gray200}
                marginTop="8px"
              >{`Gas price: ${bestShardGasPrice}`}</ParagraphXSmall>
            </div>
          </div>
          <TotalTxChart shardsGasPriceData={shardsGasPriceData} shardsTxData={shardsTxData} />
        </div>
      </ErrorBoundary>
    </InfoContainer>
  );
};
