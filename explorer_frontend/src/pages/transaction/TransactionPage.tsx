import { Meta } from "../../features/shared";
import { InternalPageContainer } from "../../features/shared/components/InternalPageContainer";
import { Layout } from "../../features/shared/components/Layout";
import { Transaction } from "../../features/transaction";

const TransactionPage = () => {
  return (
    <Layout>
      <InternalPageContainer>
        <Meta title="Transaction" description="zkSharding for Ethereum" />
        <Transaction />
      </InternalPageContainer>
    </Layout>
  );
};

export default TransactionPage;
