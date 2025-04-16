import { SPACE, Skeleton, TAG_KIND, TAG_SIZE, Tag } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { Alert } from "baseui/icon";
import { useUnit } from "effector-react";
import { useEffect, useMemo } from "react";
import { addressRoute } from "../../routing/routes/addressRoute";
import { blockRoute } from "../../routing/routes/blockRoute";
import { transactionRoute } from "../../routing/routes/transactionRoute";
import { InfoContainer, OverflowEllipsis, addHexPrefix, formatShard } from "../../shared";
import { Card } from "../../shared/components/Card";
import { Link } from "../../shared/components/Link";
import { MobileConvertableTable } from "../../shared/components/MobileConvertableTable";
import { measure } from "../../shared/utils/measure";
import { formatMethod } from "../../shared/utils/method";
import {
  $transactionList,
  type TransactionListProps,
  fetchTransactionListFx,
  showList,
} from "../model";

export const TransactionList = ({ type, identifier, view }: TransactionListProps) => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();
  useEffect(() => {
    showList({
      type,
      identifier,
    });
  }, [type, identifier]);

  const [transactions, loading] = useUnit([$transactionList, fetchTransactionListFx.pending]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: <explanation>
  const mappedTransactions = useMemo(() => {
    return transactions
      .filter((x) => x.outgoing === (view === "outgoing"))
      .map(
        ({
          from,
          to,
          hash,
          method,
          shard_id,
          block_id,
          value,
          success,
          fee_credit,
          flags,
          outgoing,
        }) => {
          return [
            <div
              key={hash}
              className={css(
                isMobile
                  ? {
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      width: "100%",
                      whiteSpace: "nowrap",
                    }
                  : {
                      display: "flex",
                      flexDirection: "row",
                      alignItems: "center",
                      flexWrap: "nowrap",
                      whiteSpace: "nowrap",
                      width: "100%",
                      overflow: "hidden",
                    },
              )}
            >
              <div
                className={css(
                  isMobile
                    ? {
                        width: "150px",
                      }
                    : {
                        flex: "0 1 auto",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      },
                )}
              >
                <Link to={transactionRoute} params={{ hash: hash.toLowerCase() }}>
                  <OverflowEllipsis>{addHexPrefix(hash.toLowerCase())}</OverflowEllipsis>
                </Link>
              </div>
              {!success && !outgoing && (
                <div
                  className={css(
                    isMobile
                      ? {
                          display: "inline",
                        }
                      : {
                          flex: "0 0 auto",
                        },
                  )}
                >
                  <Tag kind={TAG_KIND.red} size={TAG_SIZE.s}>
                    <Alert />
                  </Tag>
                </div>
              )}
            </div>,
            formatMethod(method, flags),
            <Link
              to={blockRoute}
              params={{ shard: shard_id, id: block_id }}
              key={shard_id + block_id + hash}
            >
              {formatShard(shard_id.toString(), block_id.toString())}
            </Link>,
            <Link
              to={addressRoute}
              params={{ address: from.toLowerCase() }}
              className={css({
                display: "block",
                ...(isMobile ? { width: "150px" } : {}),
              })}
              key={from.toLowerCase() + hash}
            >
              <OverflowEllipsis>{addHexPrefix(from.toLowerCase())}</OverflowEllipsis>
            </Link>,
            <Link
              to={addressRoute}
              params={{ address: to.toLowerCase() }}
              className={css({
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
                display: "block",
                ...(isMobile ? { width: "150px" } : {}),
              })}
              key={to.toLowerCase() + hash}
            >
              <OverflowEllipsis>{addHexPrefix(to.toLowerCase())}</OverflowEllipsis>
            </Link>,
            measure(value),
            measure(fee_credit),
          ];
        },
      );
  }, [transactions, view]);

  if (loading) {
    return (
      <Card
        className={css({
          marginTop: SPACE[32],
        })}
      >
        <Skeleton animation rows={4} width="300px" height="200px" />
      </Card>
    );
  }

  const columns = ["Hash", "Method", isMobile ? "Block" : "Shard + Block", "From", "To", "Value"];
  if (view === "incoming") columns.push("Fee");

  return (
    <Card
      className={css(
        isMobile && mappedTransactions.length > 0
          ? {
              height: "600px",
            }
          : {},
      )}
    >
      <InfoContainer title={isMobile ? "Transactions" : undefined}>
        <MobileConvertableTable columns={columns} data={mappedTransactions} isMobile={isMobile} />
      </InfoContainer>
    </Card>
  );
};
