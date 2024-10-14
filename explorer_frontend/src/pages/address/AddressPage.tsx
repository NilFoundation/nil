import { useUnit } from "effector-react";
import { SidebarWithBackLink, Meta, Layout } from "../../features/shared";
import { TransactionList } from "../../features/transaction-list";
import { AccountInfo } from "../../features/account";
import { useStyletron } from "baseui";
import { explorerRoute } from "../../features/routing/routes/explorerRoute";
import { addressMessagesRoute, addressRoute } from "../../features/routing";
import { HeadingXLarge, SPACE, TAB_KIND, Tab, Tabs } from "@nilfoundation/ui-kit";
import { type Store, combine } from "effector";
import type { TabsOverrides } from "baseui/tabs";

const $routes: Store<[{ address: string }, string]> = combine(
  addressRoute.$params,
  addressMessagesRoute.$params,
  addressRoute.$isOpened,
  addressMessagesRoute.$isOpened,
  (params, paramsMessages, isAddressPage, isAddressMessages) => {
    if (isAddressPage) {
      return [params, "overview"];
    }
    if (isAddressMessages) {
      return [paramsMessages, "messages"];
    }
    return [params, "overview"];
  },
);

export const AddressPage = () => {
  const [[params, key]] = useUnit([$routes]);
  const [css] = useStyletron();

  return (
    <Layout sidebar={<SidebarWithBackLink to={explorerRoute} />}>
      <Meta title={`Address ${params.address}`} description="zkSharding for Ethereum" />
      <div
        className={css({
          display: "grid",
          gridTemplateColumns: "1fr",
          width: "100%",
        })}
      >
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
            title="Messages"
            key="messages"
            onClick={(e) => {
              e.preventDefault();
              addressMessagesRoute.open(params);
            }}
            kind={TAB_KIND.secondary}
          >
            <TransactionList type="address" identifier={params.address} view="incoming" />
          </Tab>
        </Tabs>
      </div>
    </Layout>
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
