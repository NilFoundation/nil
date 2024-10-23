import { useUnit } from "effector-react";
import { useMobile } from "../../features/shared";
import { ProgressBar, PROGRESS_BAR_SIZE } from "@nilfoundation/ui-kit";
import { Code } from "../../features/code/Code";
import { Contracts } from "../../features/contracts/Contracts";
import { Logs } from "../../features/logs/Logs";
import { fetchSolidityCompiler } from "../../services/compiler";
import { useStyletron } from "baseui";
import { Navbar } from "../../features/shared/components/Layout/Navbar";
import { mobileContainerStyle, styles } from "../../features/shared/components/Layout/styles";
import { AccountPane } from "../../features/account-connector";
import { getMobileStyles } from "../../styleHelpers";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import { expandProperty } from "inline-style-expand-shorthand";
import { SandboxMobileLayout } from "./SandboxMobileLayout";
import { useEffect } from "react";
import { loadedPage } from "../../features/code/model";
import { closeApp } from "../../features/contracts/model";

export const SandboxPage = () => {
  const [isDownloading] = useUnit([fetchSolidityCompiler.pending]);
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  useEffect(() => {
    loadedPage();

    return () => {
      closeApp();
    };
  }, []);

  return (
    <div className={css(isMobile ? mobileContainerStyle : styles.container)}>
      <Navbar>
        <AccountPane />
      </Navbar>
      <div
        className={css({
          width: "100%",
          height: "calc(100vh - 90px)",
        })}
      >
        <div
          className={css({
            display: "grid",
            gridTemplateColumns: "66% 33%",
            gridTemplateRows: "1fr",
            gap: "8px",
            width: "100%",
            height: "100%",
            ...getMobileStyles({
              gridTemplateColumns: "1fr",
              gridTemplateRows: "500px",
            }),
          })}
        >
          {isMobile ? (
            <SandboxMobileLayout />
          ) : (
            <>
              <PanelGroup direction="vertical" autoSaveId="sandbox-layout">
                <Panel
                  className={css({
                    ...expandProperty("borderRadius", "12px"),
                  })}
                  minSize={10}
                  order={1}
                >
                  <Code />
                </Panel>
                <PanelResizeHandle
                  className={css({
                    height: "8px",
                  })}
                />
                <Panel
                  className={css({
                    ...expandProperty("borderRadius", "12px"),
                    overflow: "auto!important",
                  })}
                  minSize={5}
                  defaultSize={25}
                  maxSize={90}
                  order={2}
                >
                  <Logs />
                </Panel>
              </PanelGroup>
              <Contracts />
            </>
          )}
        </div>
        {isDownloading && (
          <ProgressBar
            size={PROGRESS_BAR_SIZE.large}
            minValue={0}
            maxValue={100}
            value={1}
            infinite
          />
        )}
      </div>
    </div>
  );
};
