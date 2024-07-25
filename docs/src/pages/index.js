
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageNilProducts from '@site/src/components/HomepageFeatures';



export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout
      title={`${siteConfig.title}`}
      description="The Documentation Portal for =nil; Foundation, a leading developer of zero-knowledge infrastructure and Ethereum scalability solutions.">
      <main>
        <HomepageNilProducts />
      </main>
    </Layout >
  );
}