import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import {
  areBlockchainFieldsSet,
  initializeFromStorageAndSetup,
} from "../background/storage/state.ts";
import { ErrorPage, Loading, SetEndpoint, TestnetInfo } from "../pages/get-started";
import {
  Endpoint,
  Home,
  Receive,
  Send,
  Settings,
  Testnet,
  TopUp,
  ErrorPage as WalletError,
} from "../pages/wallet";
import { ErrorScreen } from "./Error.tsx";
import { WalletRoutes } from "./routes.ts";

export const WalletRouter = () => {
  const [isFieldsSet, setIsFieldsSet] = useState<boolean | null>(null);
  const [hasError, setHasError] = useState<boolean>(false);

  const GetStartedRoutes = [
    <Route key="get-started-base" path={WalletRoutes.GET_STARTED.BASE}>
      <Route index element={<TestnetInfo />} />
      <Route path="set-endpoint" element={<SetEndpoint />} />
      <Route path="loading" element={<Loading />} />
      <Route path="error" element={<ErrorPage />} />
    </Route>,
  ];

  const WalletRoutesGroup = [
    <Route key="wallet-base" path={WalletRoutes.WALLET.BASE}>
      <Route index element={<Home />} />
    </Route>,
    <Route key="wallet-settings" path={WalletRoutes.WALLET.SETTINGS}>
      <Route index element={<Settings />} />
    </Route>,
    <Route key="wallet-receive" path={WalletRoutes.WALLET.RECEIVE}>
      <Route index element={<Receive />} />
    </Route>,
    <Route key="wallet-topup" path={WalletRoutes.WALLET.TOP_UP}>
      <Route index element={<TopUp />} />
    </Route>,
    <Route key="wallet-send" path={WalletRoutes.WALLET.SEND}>
      <Route index element={<Send />} />
    </Route>,
    <Route key="wallet-endpoint" path={WalletRoutes.WALLET.ENDPOINT}>
      <Route index element={<Endpoint />} />
    </Route>,
    <Route key="wallet-testnet" path={WalletRoutes.WALLET.TESTNET}>
      <Route index element={<Testnet />} />
    </Route>,
    <Route key="wallet-error" path={WalletRoutes.WALLET.ERROR}>
      <Route index element={<WalletError />} />
    </Route>,
  ];

  useEffect(() => {
    const checkFields = async () => {
      try {
        const fieldsSet = await areBlockchainFieldsSet();
        if (fieldsSet) {
          await initializeFromStorageAndSetup();
        }
        setIsFieldsSet(fieldsSet);
      } catch (error) {
        console.error("Error checking blockchain fields:", error);
        setHasError(true);
      }
    };

    checkFields();
  }, []);

  // If there's an error, show the error screen
  if (hasError) {
    return <ErrorScreen onRetry={() => window.location.reload()} />;
  }

  // While checking fields, render nothing
  if (isFieldsSet === null) {
    return null;
  }

  return (
    <BrowserRouter>
      {!isFieldsSet ? (
        <Routes>
          {/* Get Started Routes + Wallet Routes */}
          {GetStartedRoutes}
          {WalletRoutesGroup}
          {/* Redirect to Get Started */}
          <Route path="*" element={<Navigate to={WalletRoutes.GET_STARTED.BASE} />} />
        </Routes>
      ) : (
        <Routes>
          {/* Wallet Routes */}
          {WalletRoutesGroup}

          {/* Redirect to Wallet */}
          <Route path="*" element={<Navigate to={WalletRoutes.WALLET.BASE} />} />
        </Routes>
      )}
    </BrowserRouter>
  );
};
