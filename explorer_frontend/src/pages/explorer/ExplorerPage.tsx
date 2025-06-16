import { useEffect } from "react";
import { useStyletron } from "styletron-react";
import { Blocks } from "../../features/latest-blocks";
import { explorerRoute } from "../../features/routing";
import { Shards } from "../../features/shards";
import { Card, Heading, Layout, Meta } from "../../features/shared";
import { useMobile } from "../../features/shared";
import { Navigation } from "../../features/shared/components/Layout/Navigation";
import { TotalTransactions, TransactionHistory } from "../../features/transaction-stat";
import { explorerContainer } from "../../styleHelpers";
import { styles } from "./styles";

export const ExplorerPage = () => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  useEffect(() => {
    explorerRoute.open({});
  }, []);

  return (
    <div className={css(explorerContainer)}>
      <Layout navbar={isMobile ? null : <Navigation />}>
        <Meta title={import.meta.env.VITE_APP_TITLE} description="zkSharding for Ethereum" />
        <div className={css(isMobile ? styles.mobileContainer : styles.container)}>
          <Heading className={css(styles.heading)} />
          <Card className={css(styles.totalTrx)}>
            <TotalTransactions />
          </Card>
          <Card className={css(styles.transactionHistory)}>
            <TransactionHistory />
          </Card>
          <Card className={css(styles.shards)}>
            <Shards />
          </Card>
          <Card className={css(styles.blocks)}>
            <Blocks />
          </Card>
        </div>
      </Layout>
    </div>
  );
};
