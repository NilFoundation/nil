import { ParagraphLarge } from "baseui/typography";
import { ErrorBoundary } from "react-error-boundary";
import { useStyletron } from "styletron-react";
import { InfoContainer } from "../../../shared";
import { TotalTxChart } from "./TotalTxChart";
import { useUnit } from "effector-react";
import { $bestShard, $busiestShard, $shards, $totalTxOnShards, fetchShardsFx, fetchShrdsGasPriceFx } from "../../models/model";
import { Spinner, LabelLarge, HeadingMedium, COLORS, ParagraphXSmall } from "@nilfoundation/ui-kit";
import { formatNumber } from "../../../shared/utils/formatNumber";
import { getMobileStyles } from "../../../../styleHelpers";
import { expandProperty } from "inline-style-expand-shorthand";

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
      <ParagraphLarge>
        An error occurred while displaying the total transactions chart. Please try again later.
      </ParagraphLarge>
    </div>
  );
};

export const TotalTransactions = () => {
  const [shards, totalTxCount, busiestShard, bestShard, fetchingShardsGasPrice, fetchingShardsStat] = useUnit([
    $shards,
    $totalTxOnShards,
    $busiestShard,
    $bestShard,
    fetchShrdsGasPriceFx.pending,
    fetchShardsFx.pending,
  ]);
  const shardsCount = Object.keys(shards).length;
  const [css] = useStyletron();

  const busiestShardId = busiestShard?.[0];
  const bestShardId = bestShard?.[0];
  const busiestShardGasPrice = busiestShard?.[1] ?? "-";
  const bestShardGasPrice = bestShard?.[1] ?? "-";

  if (fetchingShardsGasPrice || fetchingShardsStat) {
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
              <HeadingMedium>{`Best shard: ${bestShardId ? `#${bestShardId}` : "-"}`}</HeadingMedium>
              <ParagraphXSmall
                color={COLORS.gray200}
                marginTop="8px"
              >{`Gas price: ${bestShardGasPrice}`}</ParagraphXSmall>
            </div>
          </div>
          <TotalTxChart />
        </div>
      </ErrorBoundary>
    </InfoContainer>
  );
};
