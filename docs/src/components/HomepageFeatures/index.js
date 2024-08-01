import styles from './styles.module.css';
import { HomepageCardSection, Card } from '../HomepageCardSection';
import Nil101 from '@site/static/img/nil/nil-icons/nil101.png';
import SmartContracts from '@site/static/img/nil/nil-icons/smart_contracts.png';
import CLI from '@site/static/img/nil/nil-icons/cli.png';
import Essentials from '@site/static/img/nil/nil-icons/essentials.png'

const HOMEPAGE_HEADER_STRING = '=nil; Documentation';
const HOMEPAGE_HEADER_SUBTITLE = '=nil; is a sharded blockchain that resolves Ethereum scalability issues via zkSharding.';

const CARDS = <HomepageCardSection>
  <Card icon={<img src={Nil101} />} id='nil101' title='=nil; 101' description='Learn how to perform essential operations on =nil;' to='./nil/getting-started/nil-101' />
  <Card icon={<img src={Essentials} />} id='essentials' title='Essentials' description='Dive into the technical intricacies of =nil;' to='./nil/getting-started/essentials/creating-a-wallet' />
  <Card icon={<img src={SmartContracts} />} id='smart_contracts' title='Tutorials' description='Write, deploy, and call smart contracts' to='./nil/getting-started/working-with-smart-contracts/writing-a-contract' />
  <Card icon={<img src={CLI} />} id='cli' title='=nil; tools' description='Configure and use the key developer tools' to='./nil/tools/nil-cli/usage' />
</HomepageCardSection>

export default function HomepageNilProducts() {
  return (
    <div className={styles.pageContainer}>
      <div className={styles.indexContainer} id='productContainer'>
        <div className='col col-2'>
          <h1 className={styles.header} style={{ textAlign: 'center' }}>
            <span>{HOMEPAGE_HEADER_STRING}</span>
          </h1>
          <h3 className={styles.subheader} style={{ textAlign: 'center', fontWeight: 'normal' }}>
            <span>{HOMEPAGE_HEADER_SUBTITLE}</span>
          </h3>
          <div className={'row ' + styles.rowFlex}>
            {CARDS}
          </div>
        </div>
      </div>
    </div>
  );
}
