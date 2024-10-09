import { useStore } from "effector-react";
import { SidebarWithBackLink } from "../../features/shared/components/SidebarWithBackLink";
import { Layout } from "../../features/shared/components/Layout";
import { BlockInfo } from "../../features/block/components/BlockInfo";
import { TransactionList } from "../../features/transaction-list";
import { blockDetailsRoute, blockRoute } from "../../features/routing/routes/blockRoute";
import { $block } from "../../features/block/models/model";
import { useStyletron } from "baseui";
import { Meta, formatShard, useMobile } from "../../features/shared";
import { explorerRoute } from "../../features/routing/routes/explorerRoute";
import {
  Tab,
  Tabs,
  HeadingXLarge,
  SPACE,
  BUTTON_SIZE,
  Button,
  BUTTON_KIND,
  Tag,
  ParagraphXSmall,
  TAB_KIND,
} from "@nilfoundation/ui-kit";
import { ArrowLeft, ArrowRight } from "baseui/icon";
import { Link } from "atomic-router-react";
import { type Store, combine } from "effector";
import type { TabsOverrides } from "baseui/tabs";

const secondary = TAB_KIND.secondary;

const $paramsStore: Store<[{ shard: string; id: string }, string]> = combine(
  blockRoute.$params,
  blockDetailsRoute.$params,
  blockRoute.$isOpened,
  blockDetailsRoute.$isOpened,
  (params, paramsDetails, isBlockPage, isBlockDetails) => {
    if (isBlockPage) {
      return [params, "overview"];
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
    return [params, "overview"];
  },
);

export const BlockPage = () => {
  const [params, key] = useStore($paramsStore);
  const block = useStore($block);
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const tabContentCn = css({
    display: "flex",
    gap: "1ch",
    alignItems: "center",
  });
  return (
    <Layout sidebar={<SidebarWithBackLink to={explorerRoute} />}>
      <Meta title="Block" description="zkSharding for Ethereum" />
      <div
        className={css({
          display: "grid",
          gridTemplateColumns: "1fr",
          width: "100%",
        })}
      >
        <div
          className={css({
            display: "flex",
            flexDirection: "row",
            justifyContent: "space-between",
            justifyItems: "flex-start",
            alignItems: "flex-start",
          })}
        >
          <HeadingXLarge className={css({ marginBottom: isMobile ? SPACE[24] : SPACE[32] })}>
            Block {formatShard(params.shard || "", params.id || "")}
          </HeadingXLarge>
          <div
            className={css({
              display: isMobile ? "none" : "flex",
              flexDirection: "row",
              rowGap: SPACE[8],
              alignItems: "flex-start",
              justifyItems: "flex-start",
            })}
          >
            {+params.id > 0 ? (
              <Link
                to={key === "overview" ? blockRoute : blockDetailsRoute}
                params={{ shard: params.shard, id: (+params.id - 1).toString(), details: key }}
              >
                <Button kind={BUTTON_KIND.tertiary} size={BUTTON_SIZE.default} startEnhancer={<ArrowLeft />}>
                  Previous block
                </Button>
              </Link>
            ) : null}
            <Link
              to={key === "overview" ? blockRoute : blockDetailsRoute}
              params={{ shard: params.shard, id: (+params.id + 1).toString(), details: key }}
            >
              <Button kind={BUTTON_KIND.tertiary} size={BUTTON_SIZE.default} endEnhancer={<ArrowRight />}>
                Next block
              </Button>
            </Link>
          </div>
        </div>
        <Tabs activeKey={key} overrides={tabsOverrides}>
          <Tab
            key={"overview"}
            kind={secondary}
            title="Overview"
            onClick={() => blockRoute.navigate({ params: { shard: params.shard, id: params.id }, query: {} })}
          >
            <BlockInfo />
          </Tab>
          <Tab
            key={"incoming"}
            kind={secondary}
            title={
              <span className={tabContentCn}>
                {isMobile ? "Incoming" : "Incoming messages"}
                <Tag>
                  <ParagraphXSmall>{block ? block.in_msg_num.padStart(3, "0") : "000"}</ParagraphXSmall>
                </Tag>
              </span>
            }
            onClick={() => {
              blockDetailsRoute.navigate({
                params: { shard: params.shard, id: params.id, details: "incoming" },
                query: {},
              });
            }}
          >
            <TransactionList type="block" identifier={`${params.shard}:${params.id}`} view="incoming" />
          </Tab>
          <Tab
            key={"outgoing"}
            kind={secondary}
            title={
              <span className={tabContentCn}>
                {isMobile ? "Outgoing" : "Outgoing messages"}
                <Tag>
                  <ParagraphXSmall>{block ? block.out_msg_num.padStart(3, "0") : "000"}</ParagraphXSmall>
                </Tag>
              </span>
            }
            onClick={() => {
              blockDetailsRoute.navigate({
                params: { shard: params.shard, id: params.id, details: "outgoing" },
                query: {},
              });
            }}
          >
            <TransactionList type="block" identifier={`${params.shard}:${params.id}`} view="outgoing" />
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
