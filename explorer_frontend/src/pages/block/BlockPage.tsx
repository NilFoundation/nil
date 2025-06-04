import {
  HeadingXLarge,
  ParagraphXSmall,
  SPACE,
  TAB_KIND,
  Tab,
  Tabs,
  Tag,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { TabsOverrides } from "baseui/tabs";
import { type Store, combine } from "effector";
import { useStore, useUnit } from "effector-react";
import { BlockInfo } from "../../features/block/components/BlockInfo";
import { $block, loadBlockFx } from "../../features/block/models/model";
import { blockDetailsRoute, blockRoute } from "../../features/routing/routes/blockRoute";
import { Meta, formatShard, useMobile } from "../../features/shared";
import { InternalPageContainer } from "../../features/shared";
import { Layout } from "../../features/shared";
import { TransactionList } from "../../features/transaction-list";

const secondary = TAB_KIND.secondary;

const $paramsStore: Store<[{ shard: string; id: string }, string]> = combine(
  blockRoute.$params,
  blockDetailsRoute.$params,
  blockRoute.$isOpened,
  blockDetailsRoute.$isOpened,
  (params, paramsDetails, isBlockPage, isBlockDetails) => {
    if (isBlockPage) {
      return [params, "all"];
    }
    if (isBlockDetails) {
      return [
        {
          shard: paramsDetails.shard,
          id: paramsDetails.id,
        },
        paramsDetails.details,
      ];
    }
    return [params, "all"];
  },
);

export const BlockPage = () => {
  const [params, key] = useStore($paramsStore);
  const [_, isPending] = useUnit([$block, loadBlockFx.pending]);
  const block = useStore($block);
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const tabContentCn = css({
    display: "flex",
    gap: "1ch",
    alignItems: "center",
  });

  return (
    <Layout>
      <Meta title="Block" description="zkSharding for Ethereum" />
      <InternalPageContainer>
        <div
          className={css({
            display: "flex",
            flexDirection: "row",
            justifyContent: "space-between",
            justifyItems: "flex-start",
            alignItems: "flex-start",
          })}
        >
          <HeadingXLarge
            className={css({
              marginBottom: isMobile ? SPACE[24] : SPACE[32],
            })}
          >
            Block {formatShard(params.shard || "", params.id || "")}
          </HeadingXLarge>
        </div>
        <div
          className={css({
            marginBlockEnd: "3rem",
          })}
        >
          <BlockInfo />
        </div>
        {!isPending && (block?.in_txn_num > 0 || block?.out_txn_num > 0) ? (
          <Tabs activeKey={key} overrides={tabsOverrides}>
            <Tab
              key={"all"}
              kind={secondary}
              title={`All ${isMobile ? "" : "transactions"}`}
              onClick={() => {
                blockDetailsRoute.navigate({
                  params: {
                    shard: params.shard,
                    id: params.id,
                    details: "all",
                  },
                  query: {},
                  replace: true,
                });
              }}
            >
              <TransactionList
                type="block"
                identifier={`${params.shard}:${params.id}`}
                view="all"
              />
            </Tab>
            <Tab
              key={"incoming"}
              kind={secondary}
              title={
                <span className={tabContentCn}>
                  {isMobile ? "Incoming" : "Incoming transactions"}
                  <Tag>
                    <ParagraphXSmall>
                      {block ? block.in_txn_num.padStart(3, "0") : "000"}
                    </ParagraphXSmall>
                  </Tag>
                </span>
              }
              onClick={() => {
                blockDetailsRoute.navigate({
                  params: {
                    shard: params.shard,
                    id: params.id,
                    details: "incoming",
                  },
                  query: {},
                  replace: true,
                });
              }}
            >
              <TransactionList
                type="block"
                identifier={`${params.shard}:${params.id}`}
                view="incoming"
              />
            </Tab>
            <Tab
              key={"outgoing"}
              kind={secondary}
              title={
                <span className={tabContentCn}>
                  {isMobile ? "Outgoing" : "Outgoing transactions"}
                  <Tag>
                    <ParagraphXSmall>
                      {block ? block.out_txn_num.padStart(3, "0") : "000"}
                    </ParagraphXSmall>
                  </Tag>
                </span>
              }
              onClick={() => {
                blockDetailsRoute.navigate({
                  params: {
                    shard: params.shard,
                    id: params.id,
                    details: "outgoing",
                  },
                  query: {},
                  replace: true,
                });
              }}
            >
              <TransactionList
                type="block"
                identifier={`${params.shard}:${params.id}`}
                view="outgoing"
              />
            </Tab>
          </Tabs>
        ) : (
          <></>
        )}
      </InternalPageContainer>
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
