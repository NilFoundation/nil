import { ErrorPage, createTheme } from "@nilfoundation/ui-kit";
import { BaseProvider } from "baseui";
import { createRoot } from "react-dom/client";
import { ErrorBoundary } from "react-error-boundary";
import { I18nextProvider } from "react-i18next";
import { Client as Styletron } from "styletron-engine-atomic";
import { Provider as StyletronProvider } from "styletron-react";
import { Container } from "./features/components/shared";
import { i18n } from "./i18n.ts";
import { WalletRouter } from "./router";

import "./features/store/model.ts";
import "./features/store/init.ts";
import "./features/utils/token.ts";

const engine = new Styletron();
const { theme } = createTheme(engine, {
  enableDefaultFonts: true,
});

const root = createRoot(document.getElementById("root") || document.body);

root.render(
  <StyletronProvider value={engine}>
    <BaseProvider theme={theme}>
      <I18nextProvider i18n={i18n}>
        <Container>
          <ErrorBoundary
            fallback={
              <ErrorPage
                errorDescription="Something went wrong... Please reload the page or try again later."
                errorCode={500}
                redirectPath="/"
                redirectTitle="Wallet Extension"
              />
            }
          >
            <WalletRouter />
          </ErrorBoundary>
        </Container>
      </I18nextProvider>
    </BaseProvider>
  </StyletronProvider>,
);
