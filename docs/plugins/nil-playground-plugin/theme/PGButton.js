import { usePluginData } from "@docusaurus/core/lib/client/exports/useGlobalData";
import { useState } from "react";
import { ThreeDots } from "react-loader-spinner";
import styles from "./styles.module.css";

const PGButton = ({ name }) => {
  const contractCodes = usePluginData("nil-playground-plugin").contractCodes;
  const code = contractCodes[name];
  const [isLoading, setIsLoading] = useState(false);

  const handleClick = async () => {
    setIsLoading(true);
    const data = await fetch("https://explore.nil.foundation/api/code.set?batch=1", {
      method: "POST",
      body: JSON.stringify({ 0: `${code}` }),
      headers: {
        "Content-Type": "application/json",
      },
    });

    const jsonResponse = await data.json();

    const hash = jsonResponse[0]?.result?.data?.hash;
    const url = `https://explore.nil.foundation/sandbox/${hash}`;

    window.open(url, "_blank");
    setIsLoading(false);
  };

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: <explanation>
    <div
      className={styles.playgroundButton}
      onClick={handleClick}
      data-goatcounter-click="Playground click"
      data-goatcounter-title={name}
    >
      {isLoading ? (
        <ThreeDots
          visible={true}
          color="#000"
          ariaLabel="three-dots-loading"
          wrapperStyle={{}}
          wrapperClass={styles.spinner}
        />
      ) : (
        "Access contract in the Playground"
      )}
    </div>
  );
};

export default PGButton;
