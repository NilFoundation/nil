import { SidebarWithBackLink } from "../../features/shared/components/SidebarWithBackLink";
import { Layout } from "../../features/shared/components/Layout";
import { Transaction } from "../../features/transaction";
import { Meta } from "../../features/shared";
import { useStyletron } from "baseui";
import { explorerRoute } from "../../features/routing/routes/explorerRoute";

const TransactionPage = () => {
  const [css] = useStyletron();
  return (
    <Layout sidebar={<SidebarWithBackLink to={explorerRoute} />}>
      <div
        className={css({
          display: "grid",
          gridTemplateColumns: "1fr",
          width: "100%",
        })}
      >
        <Meta title="Transaction" description="zkSharding for Ethereum" />
        <Transaction />
      </div>
    </Layout>
  );
};

export default TransactionPage;
