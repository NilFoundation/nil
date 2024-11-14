import styles from "./styles.module.css";
import { HomepageCardSection, Card } from "../HomepageCardSection";

const HOMEPAGE_HEADER_STRING = "=nil; Documentation";
const HOMEPAGE_HEADER_SUBTITLE =
  "=nil; is a sharded blockchain that resolves Ethereum scalability issues via zkSharding.";

const CARDS = (
  <HomepageCardSection>
    <Card
      id="nil101"
      title="=nil; 101"
      description="Learn how to perform essential operations on =nil;"
      to="https://docs.nil.foundation/nil/getting-started/nil-101"
    />
    <Card
      id="essentials"
      title="Essentials"
      description="Dive into the technical intricacies of =nil;"
      to="https://docs.nil.foundation/nil/getting-started/essentials/creating-a-wallet"
    />
    <Card
      id="guides"
      title="Guides"
      description="Explore advanced concepts and explanations"
      to="https://docs.nil.foundation/nil/guides/app-migration"
    />
    <Card
      id="cookbook"
      title="Cookbook"
      description="Access canonical examples of dApps"
      to="https://docs.nil.foundation/nil/cookbook"
    />
  </HomepageCardSection>
);

export default function HomepageNilProducts() {
  return (
    <div className={styles.pageContainer}>
      <div className={styles.indexContainer} id="productContainer">
        <div className="col col-2">
          <h1 className={styles.header} style={{ textAlign: "center" }}>
            <span>{HOMEPAGE_HEADER_STRING}</span>
          </h1>
          <h3 className={styles.subheader} style={{ textAlign: "center", fontWeight: "normal" }}>
            <span>{HOMEPAGE_HEADER_SUBTITLE}</span>
          </h3>
          <div className={"row " + styles.rowFlex}>{CARDS}</div>
        </div>
      </div>
    </div>
  );
}
