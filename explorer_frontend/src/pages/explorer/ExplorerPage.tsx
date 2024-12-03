import { useStyletron } from "styletron-react";
import { Layout, Card, Meta, Sidebar, Heading } from "../../features/shared";
import { useMobile } from "../../features/shared";
import { styles } from "./styles";
import { TransactionStat } from "../../features/transaction-stat";
import { Blocks } from "../../features/latest-blocks";
import { Shards } from "../../features/shards";
import { useEffect } from "react";
import { explorerRoute } from "../../features/routing";
import { Navigation } from "../../features/shared/components/Layout/Navigation";

const ExplorerPage = () => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  useEffect(() => {
    explorerRoute.open({});
  }, []);

  return (
    <Layout sidebar={<Sidebar />} navbar={isMobile ? null : <Navigation />}>
      <Meta title={import.meta.env.VITE_APP_TITLE} description="zkSharding for Ethereum" />
      <div className={css(isMobile ? styles.mobileContainer : styles.container)}>
        <Heading className={css(styles.heading)} />
        <Card className={css(styles.chart)}>
          <TransactionStat />
        </Card>
        <Card className={css(styles.shards)}>
          <Shards />
        </Card>
        <Card className={css(styles.blocks)}>
          <Blocks />
        </Card>
      </div>
    </Layout>
  );
};

export default ExplorerPage;
