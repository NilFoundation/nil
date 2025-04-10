import { useStyletron } from "styletron-react";
import { Meta } from "../../features/shared";
import { InternalPageContainer } from "../../features/shared/components/InternalPageContainer";
import { Layout } from "../../features/shared/components/Layout";
import { Transaction } from "../../features/transaction";
import { explorerContainer } from "../../styleHelpers";

const TransactionPage = () => {
  const [css] = useStyletron();
  return (
    <div className={css(explorerContainer)}>
      <Layout>
        <InternalPageContainer>
          <Meta title="Transaction" description="zkSharding for Ethereum" />
          <Transaction />
        </InternalPageContainer>
      </Layout>
    </div>
  );
};

export default TransactionPage;
