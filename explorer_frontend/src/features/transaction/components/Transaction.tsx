import {
  HeadingXLarge,
  ParagraphSmall,
  Skeleton,
  SPACE,
  COLORS,
  Tabs,
  Tab,
  Tag,
  TAG_SIZE,
  TAB_KIND,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { useUnit } from "effector-react";
import { $transaction, fetchTransactionFx } from "../models/transaction";
import { type Key, useState } from "react";
import type { OnChangeHandler, TabsOverrides } from "baseui/tabs";
import { Overview } from "./Overview";
import { Logs } from "./Logs";
import { $transactionLogs, fetchTransactionLogsFx } from "../models/transactionLogs";
import { TransactionList } from "../../transaction-list";
import { useMobile } from "../../shared";

export const Transaction = () => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const [transaction, pending] = useUnit([$transaction, fetchTransactionFx.pending]);
  const [logs, logsPending] = useUnit([$transactionLogs, fetchTransactionLogsFx.pending]);
  const [activeKey, setActiveKey] = useState<Key>("0");
  const onChangeHandler: OnChangeHandler = (currentKey) => {
    setActiveKey(currentKey.activeKey);
  };

  return (
    <>
      <HeadingXLarge className={css({ marginBottom: SPACE[32] })}>Message</HeadingXLarge>
      {!transaction ? (
        pending ? (
          <Skeleton animation />
        ) : (
          <ParagraphSmall color={COLORS.gray100}>Message not found</ParagraphSmall>
        )
      ) : (
        <Tabs activeKey={activeKey} onChange={onChangeHandler} overrides={tabsOverrides}>
          <Tab title="Overview" kind={TAB_KIND.secondary}>
            <Overview transaction={transaction} />
          </Tab>
          <Tab
            title="Logs"
            endEnhancer={<Tag size={TAG_SIZE.m}>{logs?.length ?? 0}</Tag>}
            kind={TAB_KIND.secondary}
          >
            {logsPending ? <Skeleton animation /> : <Logs logs={logs} />}
          </Tab>
          <Tab title={isMobile ? "Outgoing msg" : "Outgoing messages"} kind={TAB_KIND.secondary}>
            <TransactionList type="transaction" identifier={transaction.hash} view="incoming" />
          </Tab>
        </Tabs>
      )}
    </>
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
