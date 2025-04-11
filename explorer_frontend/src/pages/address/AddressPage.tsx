import { HeadingXLarge, SPACE, TAB_KIND, Tab, Tabs } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { TabsOverrides } from "baseui/tabs";
import { type Store, combine } from "effector";
import { useUnit } from "effector-react";
import { AccountInfo } from "../../features/account";
import { addressRoute, addressTransactionsRoute } from "../../features/routing";
import { Layout, Meta } from "../../features/shared";
import { InternalPageContainer } from "../../features/shared";
import { TransactionList } from "../../features/transaction-list";
import { explorerContainer } from "../../styleHelpers";

const $routes: Store<[{ address: string }, string]> = combine(
  addressRoute.$params,
  addressTransactionsRoute.$params,
  addressRoute.$isOpened,
  addressTransactionsRoute.$isOpened,
  (params, paramsTransactions, isAddressPage, isAddressTransactions) => {
    if (isAddressPage) {
      return [params, "overview"];
    }
    if (isAddressTransactions) {
      return [paramsTransactions, "transactions"];
    }
    return [params, "overview"];
  },
);

export const AddressPage = () => {
  const [[params, key]] = useUnit([$routes]);
  const [css] = useStyletron();

  return (
    <div className={css(explorerContainer)}>
      <Layout>
        <Meta title={`Address ${params.address}`} description="zkSharding for Ethereum" />
        <InternalPageContainer>
          <HeadingXLarge className={css({ marginBottom: SPACE[32], wordBreak: "break-word" })}>
            Account {params.address}
          </HeadingXLarge>
          <Tabs activeKey={key} overrides={tabsOverrides}>
            <Tab
              title="Overview"
              key="overview"
              onClick={(e) => {
                e.preventDefault();
                addressRoute.open(params);
              }}
              kind={TAB_KIND.secondary}
            >
              <AccountInfo />
            </Tab>
            <Tab
              title="Transactions"
              key="transactions"
              onClick={(e) => {
                e.preventDefault();
                addressTransactionsRoute.open(params);
              }}
              kind={TAB_KIND.secondary}
            >
              <TransactionList type="address" identifier={params.address} view="incoming" />
            </Tab>
          </Tabs>
        </InternalPageContainer>
      </Layout>
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
