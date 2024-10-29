import React from 'react';
import { usePluginData } from '@docusaurus/core/lib/client/exports/useGlobalData';
import styles from './styles.module.css';

const PGButton = ({ name }) => {
  const contractCodes = usePluginData('nil-playground-plugin').contractCodes;
  console.log(contractCodes);
  const code = contractCodes[name];

  const handleClick = async () => {
    const data = await fetch('https://explore.nil.foundation/api/code.set?batch=1', {
      method: 'POST',
      body: JSON.stringify({ 0: `${code}` }),
      headers: {
        'Content-Type': 'application/json',
      },
    });

    const jsonResponse = await data.json();

    const hash = jsonResponse[0]?.result?.data?.hash;
    const url = `https://explore.nil.foundation/sandbox/${hash}`;

    window.open(url, '_blank');
  };

  return (
    <div className={styles.playgroundButton} onClick={handleClick}>
      Access contract in the Playground
    </div>
  );
};

export default PGButton;
