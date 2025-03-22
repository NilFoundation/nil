import { useStyletron } from "baseui";
import { Meta } from "../../features/shared";
import { Layout } from "../../features/shared/components/Layout";
import { Transaction } from "../../features/transaction";

const TransactionPage = () => {
  const [css] = useStyletron();
  return (
    <Layout>
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
