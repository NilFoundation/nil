import {
  COLORS,
  HeadingXLarge,
  ParagraphSmall,
  SPACE,
  Skeleton,
  TAB_KIND,
  TAG_SIZE,
  Tab,
  Tabs,
  Tag,
} from "@nilfoundation/ui-kit";
import type { OnChangeHandler, TabsOverrides } from "baseui/tabs";
import { useUnit } from "effector-react";
import { type Key, useEffect, useState } from "react";
import { useStyletron } from "styletron-react";
import { useMobile } from "../../shared";
import { TransactionList } from "../../transaction-list";
import { $transaction, fetchTransactionFx } from "../models/transaction";
import { $transactionChilds, fetchTransactionChildsFx } from "../models/transactionChilds.ts";
import { $transactionLogs, fetchTransactionLogsFx } from "../models/transactionLogs";
import { Logs } from "./Logs";
import { Overview } from "./Overview";

export const Transaction = () => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const [transaction, pending] = useUnit([$transaction, fetchTransactionFx.pending]);
  const [logs, logsPending] = useUnit([$transactionLogs, fetchTransactionLogsFx.pending]);
  const [transactionChilds, childsLoading] = useUnit([
    $transactionChilds,
    fetchTransactionChildsFx.pending,
  ]);

  useEffect(() => {
    if (transaction) setActiveKey("0");
  }, [transaction]);

  const [activeKey, setActiveKey] = useState<Key>("0");
  const onChangeHandler: OnChangeHandler = (currentKey) => {
    setActiveKey(currentKey.activeKey);
  };
  useEffect(() => {
    if (transaction) {
      fetchTransactionChildsFx(transaction.hash);
    }
  }, [transaction]);

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        flexWrap: "nowrap",
        width: "100%",
      })}
    >
      <HeadingXLarge className={css({ marginBottom: SPACE[32] })}>Transaction</HeadingXLarge>
      {!transaction ? (
        pending ? (
          <Skeleton animation />
        ) : (
          <ParagraphSmall color={COLORS.gray100}>Transaction not found</ParagraphSmall>
        )
      ) : (
        <Tabs activeKey={activeKey} onChange={onChangeHandler} overrides={tabsOverrides}>
          <Tab title="Overview" kind={TAB_KIND.secondary}>
            <Overview transaction={transaction} />
          </Tab>
          <Tab
            title="Logs"
            endEnhancer={logs?.length > 0 ? <Tag size={TAG_SIZE.m}>{logs?.length ?? 0}</Tag> : ""}
            kind={TAB_KIND.secondary}
          >
            {logsPending ? <Skeleton animation /> : <Logs logs={logs} />}
          </Tab>
          {!childsLoading && transactionChilds?.length > 0 ? (
            <Tab
              title={isMobile ? "Outgoing txn" : "Outgoing transactions"}
              endEnhancer={
                <Tag size={TAG_SIZE.m}>{!childsLoading ? transactionChilds?.length : "..."}</Tag>
              }
              kind={TAB_KIND.secondary}
            >
              <TransactionList type="transaction" identifier={transaction.hash} view="incoming" />
            </Tab>
          ) : (
            <></>
          )}
        </Tabs>
      )}
    </div>
  );
};

const tabsOverrides: TabsOverrides = {
  TabContent: {
    style: {
      paddingLeft: 0,
      paddingRight: 0,
    },
  },
  TabBar: {
    style: {
      paddingLeft: 0,
      paddingRight: 0,
    },
  },
};
