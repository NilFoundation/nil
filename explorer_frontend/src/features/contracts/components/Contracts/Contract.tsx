import type { Hex } from "@nilfoundation/niljs";
import {
  BUTTON_KIND,
  Button,
  COLORS,
  CopyButton,
  HeadingMedium,
  SPACE,
} from "@nilfoundation/ui-kit";
import { expandProperty } from "inline-style-expand-shorthand";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import type { App } from "../../../../types";
import { choseApp } from "../../models/base";
import { RemoveAppButton } from "../RemoveAppButton";

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
                <RemoveAppButton address={address!} bytecode={bytecode} disabled={!address} />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
