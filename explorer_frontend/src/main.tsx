import "./init";
import { createRoot } from "react-dom/client";
import { Provider as StyletronProvider } from "styletron-react";
import { App } from "./App";
import { ThemedProvider } from "./ThemedProvider";
import { engine } from "./themes";

const root = createRoot(document.getElementById("root") || document.body);

root.render(
  <StyletronProvider value={engine}>
    <ThemedProvider>
      <App />
    </ThemedProvider>
  </StyletronProvider>,
);
