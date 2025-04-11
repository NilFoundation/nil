import {
  COLORS,
  CopyButton,
  SPACE,
  Skeleton,
  TAG_KIND,
  TAG_SIZE,
  Tag,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { useEffect, useMemo, useState } from "react";
import { addressRoute } from "../../routing/routes/addressRoute";
import { blockRoute } from "../../routing/routes/blockRoute";
import { transactionRoute } from "../../routing/routes/transactionRoute";
import {
  DangerIcon,
  InfoContainer,
  OverflowEllipsis,
  RightArrowIcon,
  addHexPrefix,
  formatShard,
} from "../../shared";
import { Card } from "../../shared/components/Card";
import { Link } from "../../shared/components/Link";
import { useMobile } from "../../shared/hooks/useMobile";
import { measure } from "../../shared/utils/measure";
import { formatMethod } from "../../shared/utils/method";
import {
  $transactionList,
  type TransactionListProps,
  fetchTransactionListFx,
  showList,
} from "../model";
import { EmptyTransaction } from "./EmptyTransactionList";
import { Paginate } from "./Pagination";
import { TransactionsTable } from "./TransactionsTable";

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

  const [currentPage, setCurrentPage] = useState(1);
  const transactionsPerPage = 10;

  const filteredTransactions = useMemo(() => {
    return transactions.filter((tx) => {
      if (view === "all") {
        return true;
      }
      return tx.outgoing === (view === "outgoing");
    });
  }, [transactions, view]);

  const totalPages = useMemo(() => {
    return Math.ceil(filteredTransactions.length / transactionsPerPage);
  }, [filteredTransactions.length, transactionsPerPage]);

  useEffect(() => {
    setCurrentPage(1);
  }, [view]);

  const paginatedTransactions = useMemo(() => {
    const startIndex = (currentPage - 1) * transactionsPerPage;
    const endIndex = startIndex + transactionsPerPage;
    return filteredTransactions.slice(startIndex, endIndex);
  }, [filteredTransactions, currentPage, transactionsPerPage]);

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  const mappedTransactions = useMemo(() => {
    return paginatedTransactions.map(
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
        return {
          hash: (
            <div
              key={hash}
              className={css({
                display: "flex",
                flexDirection: "row",
                alignItems: "center",
                flexWrap: "nowrap",
                whiteSpace: "nowrap",
                width: "100%",
                overflow: "hidden",
              })}
            >
              <div
                className={css({
                  display: "flex",
                  flexDirection: "row",
                  alignItems: "center",
                  whiteSpace: "nowrap",
                  overflow: "hidden",
                  flex: "1 1 auto",
                })}
              >
                <div
                  className={css({
                    flex: "1 1 auto",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                  })}
                >
                  <Link to={transactionRoute} params={{ hash: hash.toLowerCase() }}>
                    <OverflowEllipsis>{addHexPrefix(hash.toLowerCase())}</OverflowEllipsis>
                  </Link>
                </div>
                {!success && !outgoing && (
                  <div
                    className={css({
                      marginLeft: "8px",
                      marginRight: "8px",
                      flexShrink: 0,
                    })}
                  >
                    <Tag kind={TAG_KIND.red} size={TAG_SIZE.s}>
                      <DangerIcon />
                    </Tag>
                  </div>
                )}
                <CopyButton
                  textToCopy={hash.toLowerCase()}
                  disabled={hash === ""}
                  color={COLORS.gray100}
                />
              </div>
            </div>
          ),
          method: formatMethod(method, flags),
          shardBlock: (
            <Link
              to={blockRoute}
              params={{ shard: shard_id, id: block_id }}
              key={shard_id + block_id + hash}
            >
              {formatShard(shard_id.toString(), block_id.toString())}
            </Link>
          ),
          from: (
            <div
              className={css({
                display: "flex",
                alignItems: "center",
                gap: "2px",
                textOverflow: "ellipsis",
              })}
            >
              <Link
                to={addressRoute}
                params={{ address: from.toLowerCase() }}
                className={css({
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  display: "block",
                  ...(isMobile ? { width: "150px" } : {}),
                })}
                key={from.toLowerCase() + hash}
              >
                <OverflowEllipsis>{addHexPrefix(from.toLowerCase())}</OverflowEllipsis>
              </Link>
              <CopyButton textToCopy={from.toLowerCase()} />
            </div>
          ),
          arrow: <RightArrowIcon />,
          to: (
            <div
              className={css({
                display: "flex",
                alignItems: "center",
                gap: "8px",
                textOverflow: "ellipsis",
              })}
            >
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
              </Link>
              <CopyButton textToCopy={to.toLowerCase()} />
            </div>
          ),
          value: measure(value),
          fee: measure(fee_credit),
        };
      },
    );
  }, [paginatedTransactions, css, isMobile]);

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

  const columns = ["Hash", "Method", "Shard + Block", "From", "", "To", "Value", "Fee"];
  if (view === "outgoing") columns.pop();

  return (
    <Card
      className={css(
        isMobile && mappedTransactions.length > 0
          ? {
              width: "90vw",
              overflowX: "scroll",
              marginInline: "auto",
            }
          : {},
      )}
    >
      {filteredTransactions.length > 0 ? (
        <InfoContainer title={undefined}>
          <TransactionsTable columns={columns} data={mappedTransactions} view={view} />
          <div
            className={css({
              display: "flex",
              justifyContent: "flex-end",
              marginTop: SPACE[16],
            })}
          >
            {totalPages > 1 && (
              <Paginate
                currentPage={currentPage}
                totalPages={totalPages}
                onPageChange={handlePageChange}
              />
            )}
          </div>
        </InfoContainer>
      ) : (
        <EmptyTransaction />
      )}
    </Card>
  );
};
