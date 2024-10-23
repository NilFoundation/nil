import { useStyletron } from "styletron-react";
import { styles } from "./styles";
import { LabelMedium } from "baseui/typography";
import { getBackgroundBasedOnWorkload, type SHARD_WORKLOAD } from "../../types/SHARD_WORKLOAD";
import { formatNumber } from "../../../shared";

type ShardProps = {
  workload: SHARD_WORKLOAD;
  txCount: number;
};

export const Shard = ({ workload, txCount }: ShardProps) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({ ...styles.shard, backgroundColor: getBackgroundBasedOnWorkload(workload) })}
    >
      <LabelMedium>{formatNumber(txCount)}</LabelMedium>
    </div>
  );
};
