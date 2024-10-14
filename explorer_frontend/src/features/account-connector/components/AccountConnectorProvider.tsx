import { useMemo, type ReactNode } from "react";
import { $endpoint, $privateKey, $wallet } from "../models/model";
import { useUnit } from "effector-react";
import { AccountConnectorContext } from "./AccountContext";
import { HttpTransport, LocalECDSAKeySigner } from "@nilfoundation/niljs";

type AccountConnectorProps = {
  children: ReactNode;
};

const AccountConnectorProvider = ({ children }: AccountConnectorProps) => {
  const [privateKey, endpoint, wallet] = useUnit([$privateKey, $endpoint, $wallet]);

  const signer = useMemo(() => {
    if (!privateKey) {
      return null;
    }

    return new LocalECDSAKeySigner({ privateKey });
  }, [privateKey]);

  const transport = useMemo(() => {
    if (!endpoint) {
      return null;
    }
    return new HttpTransport({ endpoint });
  }, [endpoint]);

  const value = useMemo(() => {
    return {
      signer,
      transport,
      wallet,
    };
  }, [signer, transport, wallet]);

  return <AccountConnectorContext.Provider value={value}>{children}</AccountConnectorContext.Provider>;
};

export { AccountConnectorProvider };
