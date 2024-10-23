import type { FC } from "react";
import type { App } from "../../types";
import { useStyletron } from "styletron-react";
import {
  BUTTON_KIND,
  Button,
  ButtonIcon,
  COLORS,
  CopyButton,
  HeadingMedium,
  SPACE,
  StatefulTooltip,
} from "@nilfoundation/ui-kit";
import { choseApp, unlinkApp } from "./model";
import { expandProperty } from "inline-style-expand-shorthand";
import type { Hex } from "@nilfoundation/niljs";
import { DeleteIcon } from "./DeleteIcon";

type ContractProps = {
  contract: App;
  deployedApps: Array<App & { address?: Hex }>;
};

export const Contract: FC<ContractProps> = ({ contract, deployedApps }) => {
  const [css] = useStyletron();

  return (
    <div
      key={contract.bytecode}
      className={css({
        background: "transparent",
        ...expandProperty("padding", "12px 0"),
        display: "flex",
        flexDirection: "column",
        ":not(:last-child)": {
          borderBottom: `1px solid ${COLORS.gray700}`,
        },
      })}
    >
      <div
        className={css({
          display: "flex",
          flexDirection: "row",
          justifyContent: "space-between",
          alignItems: "center",
          paddingLeft: "8px",
          paddingRight: "8px",
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
          kind={BUTTON_KIND.primary}
        >
          Deploy
        </Button>
      </div>
      <div
        className={css({
          display: "flex",
          flexDirection: "column",
          gap: "12px",
          paddingLeft: "8px",
          paddingRight: "8px",
        })}
      >
        {deployedApps.map(({ address, bytecode }) => {
          return (
            <div
              className={css({
                display: "flex",
                height: "48px",
                flexDirection: "row",
                alignItems: "center",
                gap: "8px",
                backgroundColor: COLORS.gray800,
                ...expandProperty("padding", "12px 16px"),
                ...expandProperty("borderRadius", "8px"),
                ...expandProperty("transition", "background-color 0.15s ease-in"),
                ":hover": {
                  backgroundColor: COLORS.gray700,
                },
                cursor: "pointer",
                ":first-child": {
                  marginTop: "12px",
                },
              })}
              key={address}
              onClick={() => {
                choseApp({ address, bytecode });
              }}
              onKeyDown={() => {
                choseApp({ address, bytecode });
              }}
            >
              <div
                className={css({
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whitespace: "nowrap",
                  flexGrow: "2",
                  color: COLORS.gray200,
                })}
              >
                {address}
              </div>
              <div
                className={css({
                  display: "flex",
                  alignItems: "center",
                  flexGrow: "0",
                })}
              >
                <CopyButton
                  overrides={{
                    Root: {
                      style: {
                        height: "40px",
                        width: "40px",
                      },
                    },
                  }}
                  textToCopy={address ?? ""}
                  onClick={(e) => e.stopPropagation()}
                  onKeyDown={(e) => e.stopPropagation()}
                />
                <StatefulTooltip
                  content="Remove app"
                  showArrow={false}
                  placement="bottom"
                  popoverMargin={0}
                >
                  <ButtonIcon
                    icon={<DeleteIcon />}
                    kind={BUTTON_KIND.text}
                    onClick={(e) => {
                      e.stopPropagation();
                      if (address)
                        unlinkApp({
                          app: bytecode,
                          address: address,
                        });
                    }}
                    onKeyDown={(e) => {
                      e.stopPropagation();
                      if (!(e.key === "Enter" || e.key === " ")) {
                        return;
                      }

                      if (address)
                        unlinkApp({
                          app: bytecode,
                          address: address,
                        });
                    }}
                  />
                </StatefulTooltip>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
