import { PROGRESS_BAR_SIZE, ProgressBar, Tab, Tabs } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { useEffect, useState } from "react";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import { AccountPane } from "../../features/account-connector";
import { Code } from "../../features/code/Code";
import { loadedTutorialPage } from "../../features/code/model";
import { closeApp } from "../../features/contracts";
import { ContractsContainer } from "../../features/contracts/components/ContractsContainer";
import { NetworkErrorNotification } from "../../features/healthcheck";
import { $rpcIsHealthy } from "../../features/healthcheck/model";
import { Logs } from "../../features/logs/components/Logs";
import { useMobile } from "../../features/shared";
import { Navbar } from "../../features/shared/components/Layout/Navbar";
import { mobileContainerStyle, styles } from "../../features/shared/components/Layout/styles";
import { TutorialText } from "../../features/tutorial/TutorialText";
import { fetchSolidityCompiler } from "../../services/compiler";
import { TutorialMobileLayout } from "./TutorialMobileLayout";
import "./init.ts";
import { $tutorialChecksState } from "./model.tsx";

export const TutorialPage = () => {
  const [isDownloading, isRPCHealthy] = useUnit([fetchSolidityCompiler.pending, $rpcIsHealthy]);
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  const [activeKey, setActiveKey] = useState("0");

  const [tutorialChecks] = useUnit([$tutorialChecksState]);

  useEffect(() => {
    loadedTutorialPage(true);

    return () => {
      closeApp();
    };
  }, []);

  return (
    <div className={css(isMobile ? mobileContainerStyle : styles.container)}>
      {!isRPCHealthy && <NetworkErrorNotification />}
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
            width: "100%",
            height: "100%",
          })}
        >
          {isMobile ? (
            <TutorialMobileLayout />
          ) : (
            <>
              <PanelGroup direction="horizontal" autoSaveId="playground-layout-horizontal">
                <Panel>
                  <PanelGroup direction="vertical" autoSaveId="playground-layout-vertical">
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
                </Panel>
                <PanelResizeHandle
                  className={css({
                    width: "8px",
                  })}
                />
                <Panel minSize={20} defaultSize={33} maxSize={90}>
                  <Tabs
                    onChange={({ activeKey }) => {
                      setActiveKey(activeKey);
                    }}
                    activeKey={activeKey}
                    overrides={{
                      Root: {
                        style: {
                          height: "100%",
                          display: "flex",
                          flexDirection: "column",
                        },
                      },
                      TabContent: {
                        style: {
                          height: "100%",
                          flex: "1 1 auto",
                        },
                      },
                      TabBar: {
                        style: {
                          display: "flex",
                          justifyContent: "center",
                          alignItems: "center",
                        },
                      },
                      Tab: {
                        style: {
                          flex: 1,
                          display: "flex",
                          textAlign: "center",
                          alignContent: "center",
                          justifyContent: "center",
                          fontSize: "16px",
                          fontWeight: "400",
                        },
                      },
                    }}
                  >
                    <Tab title="Tutorial">
                      <TutorialText />
                    </Tab>
                    <Tab title="Contracts" disabled={!tutorialChecks}>
                      <ContractsContainer />
                    </Tab>
                  </Tabs>
                </Panel>
              </PanelGroup>
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
