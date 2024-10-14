import type { ISigner, ITransport, WalletV1 } from "@nilfoundation/niljs";
import { createContext } from "react";

type AccountConnectorContextType = {
  signer: ISigner | null;
  transport: ITransport | null;
  wallet: WalletV1 | null;
};

export const AccountConnectorContext = createContext<AccountConnectorContextType>({
  signer: null,
  transport: null,
  wallet: null,
});
