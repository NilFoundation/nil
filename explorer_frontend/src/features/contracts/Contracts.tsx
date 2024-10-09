import { useUnit } from "effector-react";
import { $activeAppWithState, $contractWithState, $contracts, choseApp, closeApp } from "./model";
import "./init";
import {
  BUTTON_KIND,
  BUTTON_SIZE,
  HeadingMedium,
  Modal,
  SPACE,
  Button,
  Card,
  COLORS,
  LabelMedium,
  Spinner,
  ArrowUpIcon,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { DeployForm } from "./DeployForm";
import { ContractManagement } from "./ContractManagement";
import { compileCodeFx } from "../code/model";
import { useMobile } from "../shared";
import { getMobileStyles } from "../../styleHelpers";
import { LayoutComponent, setActiveComponent } from "../../pages/sandbox/model";

export const Contracts = () => {
  const [deployedApps, activeApp, contracts, compilingContracts] = useUnit([
    $contractWithState,
    $activeAppWithState,
    $contracts,
    compileCodeFx.pending,
  ]);
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        ...getMobileStyles({
          height: "calc(100vh - 109px)",
        }),
      })}
    >
      {isMobile && (
        <div
          className={css({
            display: "flex",
            gap: "12px",
            marginBottom: SPACE[12],
            alignItems: "center",
          })}
        >
          <Button
            className={css({
              width: "32px",
              height: "32px",
            })}
            overrides={{
              Root: {
                style: {
                  paddingLeft: 0,
                  paddingRight: 0,
                },
              },
            }}
            kind={BUTTON_KIND.secondary}
            size={BUTTON_SIZE.compact}
            onClick={() => setActiveComponent(LayoutComponent.Code)}
          >
            <ArrowUpIcon
              size={12}
              className={css({
                transform: "rotate(-90deg)",
              })}
            />
          </Button>
          <LabelMedium color={COLORS.gray50}>Contracts</LabelMedium>
        </div>
      )}
      <Card
        overrides={{
          Root: {
            style: {
              maxWidth: "none",
              height: "100%",
              backgroundColor: COLORS.gray900,
            },
          },
          Contents: {
            style: {
              height: "100%",
              maxWidth: "none",
              width: "100%",
            },
          },
          Body: {
            style: {
              height: "100%",
              width: "100%",
              maxWidth: "none",
            },
          },
        }}
      >
        {contracts.length === 0 && (
          <div
            className={css({
              height: "100%",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              paddingLeft: "25%",
              paddingRight: "25%",
              textAlign: "center",
            })}
          >
            {compilingContracts ? (
              <Spinner />
            ) : (
              <LabelMedium color={COLORS.gray400}>Compile the code to handle smart contracts.</LabelMedium>
            )}
          </div>
        )}
        {contracts.map((contract) => {
          return (
            <div
              key={contract.bytecode}
              className={css({
                background: "#212121",
                borderRadius: "8px",
                padding: SPACE[8],
                display: "flex",
                flexDirection: "column",
              })}
            >
              <Modal
                isOpen={!!activeApp}
                onClose={() => {
                  closeApp();
                }}
                size={"80vw"}
              >
                {activeApp?.address ? <ContractManagement /> : <DeployForm />}
              </Modal>
              <div
                className={css({
                  display: "flex",
                  flexDirection: "row",
                  justifyContent: "space-between",
                  alignItems: "center",
                  paddingBottom: SPACE[8],
                })}
              >
                <HeadingMedium
                  className={css({
                    wordBreak: "break-word",
                    paddingRight: SPACE[8],
                  })}
                >
                  {contract.name}
                </HeadingMedium>
                <Button
                  onClick={() => {
                    choseApp({
                      bytecode: contract.bytecode,
                    });
                  }}
                  size={BUTTON_SIZE.compact}
                  kind={BUTTON_KIND.primary}
                >
                  Deploy
                </Button>
              </div>
              <div>
                {deployedApps
                  .filter((app) => app.bytecode === contract.bytecode)
                  .map(({ address, bytecode }) => {
                    return (
                      <Button
                        overrides={{
                          Root: {
                            style: {
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whitespace: "nowrap",
                              marginBottom: SPACE[8],
                              width: "100%",
                              display: "inline-block",
                            },
                          },
                        }}
                        key={address}
                        size={BUTTON_SIZE.compact}
                        kind={BUTTON_KIND.secondary}
                        onClick={() => {
                          choseApp({ address, bytecode });
                        }}
                      >
                        {address}
                      </Button>
                    );
                  })}
                {deployedApps.filter((app) => app.bytecode === contract.bytecode).length === 0 && (
                  <Button
                    overrides={{
                      Root: {
                        style: {
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whitespace: "nowrap",
                          marginBottom: SPACE[8],
                          width: "100%",
                          display: "inline-block",
                        },
                      },
                    }}
                    size={BUTTON_SIZE.compact}
                    kind={BUTTON_KIND.secondary}
                    onClick={() => {
                      choseApp({ bytecode: contract.bytecode });
                    }}
                  >
                    Deploy new smart contract
                  </Button>
                )}
              </div>
            </div>
          );
        })}
      </Card>
    </div>
  );
};
