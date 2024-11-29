import type { FC } from "react";
import { styled, useStyletron } from "styletron-react";
import type { Hex } from "@nilfoundation/niljs";
import { useRepeatedGetTxByHash } from "../hooks/useRepeatedGetTxByHash";
import { COLORS, MonoLabelMedium, Spinner } from "@nilfoundation/ui-kit";
import { formatShard, measure } from "../../shared";

type TxDetialsProps = {
  txHash: Hex;
};

const StyledLabel = styled(MonoLabelMedium, {
  color: COLORS.gray400,
  whiteSpace: "nowrap",
});

export const TxDetials: FC<TxDetialsProps> = ({ txHash }) => {
  const [css] = useStyletron();
  const { data: tx, loading, error } = useRepeatedGetTxByHash(txHash);

  return (
    <div
      className={css({
        display: "grid",
        gridTemplateColumns: "min-content 1fr",
        rowGap: "8px",
        columnGap: "16px",
      })}
    >
      {loading && !tx && !error && (
        <div
          className={css({
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            gridColumn: "1 / 3",
            paddingTop: "8px",
            paddingBottom: "8px",
          })}
        >
          <Spinner />
        </div>
      )}
      {error && <div>Error</div>}
      {tx && (
        <>
          <StyledLabel>shard + block height:</StyledLabel>
          <StyledLabel>{formatShard(tx.shard_id.toString(), tx.block_id.toString())}</StyledLabel>
          <StyledLabel>nonce:</StyledLabel>
          <StyledLabel>{tx.seqno}</StyledLabel>
          <StyledLabel>status:</StyledLabel>
          <StyledLabel>{tx.success ? "Success" : "Failed"}</StyledLabel>
          <StyledLabel>from:</StyledLabel>
          <StyledLabel>{tx.from}</StyledLabel>
          <StyledLabel>to:</StyledLabel>
          <StyledLabel>{tx.to}</StyledLabel>
          <StyledLabel>value:</StyledLabel>
          <StyledLabel>{measure(tx.value)}</StyledLabel>
          <StyledLabel>Fee credit:</StyledLabel>
          <StyledLabel>{measure(tx.fee_credit)}</StyledLabel>
          <StyledLabel>Gas used:</StyledLabel>
          <StyledLabel>{tx.gas_used ?? 0}</StyledLabel>
        </>
      )}
    </div>
  );
};
