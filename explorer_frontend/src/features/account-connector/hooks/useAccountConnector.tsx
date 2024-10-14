import { useContext } from "react";
import { AccountConnectorContext } from "../components/AccountContext";

const useAccountConnector = () => {
  if (!AccountConnectorContext) {
    throw new Error("AccountConnectorContext is not provided. Use this hook inside AccountConnector component.");
  }

  const { signer, transport, wallet } = useContext(AccountConnectorContext);

  return { signer, transport, wallet };
};

export { useAccountConnector };
