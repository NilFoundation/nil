import {
  BUTTON_KIND,
  ButtonIcon,
  COLORS,
  CopyButton,
  LabelMedium,
  ParagraphMedium,
  StatefulTooltip,
} from "@nilfoundation/ui-kit";
import type { AbiFunction } from "abitype";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { useMemo } from "react";
import { formatEther } from "viem";
import { addressRoute } from "../../../routing";
import { Link, ShareIcon, useMobile } from "../../../shared";
import { getTokenSymbolByAddress } from "../../../tokens";
import {
  $activeAppWithState,
  $activeKeys,
  $balance,
  $callParams,
  $callResult,
  $errors,
  $loading,
  $tokens,
  $txHashes,
  setParams,
} from "../../models/base";
import { ContractManagementHeader } from "./ContractManagementHeader";
import { Method } from "./Method";

export const ContractManagement = () => {
  const [app, activeKeys, balance, tokens, callParams, callResult, loading, errors, txHashes] =
    useUnit([
      $activeAppWithState,
      $activeKeys,
      $balance,
      $tokens,
      $callParams,
      $callResult,
      $loading,
      $errors,
      $txHashes,
    ]);
  const [isMobile] = useMobile();
  const [css, theme] = useStyletron();
  const functions = useMemo(() => {
    if (!app) {
      return [];
    }

    return app.abi.filter((abiField) => {
      return abiField.type === "function";
    }) as AbiFunction[];
  }, [app]);

  return (
    <>
      <ContractManagementHeader
        address={app?.address!}
        bytecode={app?.bytecode!}
        name={app?.name!}
      />
      <div
        className={css({
          paddingTop: "24px",
          paddingBottom: "24px",
          display: "grid",
          columnGap: "24px",
          rowGap: "16px",
          minWidth: isMobile ? "none" : "300px",
          maxWidth: isMobile ? "100%" : "none",
          gridTemplateColumns: "auto 1fr auto",
          gridTemplateRows: "auto",
        })}
      >
        <LabelMedium
          className={css({
            gridColumn: "1 / 2",
            gridRow: "1 / 2",
          })}
          color={COLORS.gray400}
        >
          Address:
        </LabelMedium>
        <LabelMedium
          className={css({
            gridColumn: "2 / 3",
            gridRow: "1 / 2",
            wordBreak: "break-all",
          })}
          color={COLORS.gray50}
        >
          {app?.address}
        </LabelMedium>
        <div
          className={css({
            gridColumn: "3 / 4",
            gridRow: "1 / 2",
            display: "flex",
            gap: "8px",
          })}
        >
          <CopyButton
            kind={BUTTON_KIND.secondary}
            overrides={{
              Root: {
                style: {
                  height: "32px",
                  width: "32px",
                  backgroundColor: theme.colors.backgroundSecondary,
                  ":hover": {
                    backgroundColor: theme.colors.backgroundTertiary,
                  },
                },
              },
            }}
            textToCopy={app?.address ?? ""}
          />
          <Link to={addressRoute} params={{ address: app?.address }} target="_blank">
            <StatefulTooltip content="Open in Explorer" showArrow={false} placement="bottom">
              <ButtonIcon
                overrides={{
                  Root: {
                    style: {
                      height: "32px",
                      width: "32px",
                      backgroundColor: theme.colors.backgroundSecondary,
                      ":hover": {
                        backgroundColor: theme.colors.backgroundTertiary,
                      },
                    },
                  },
                }}
                kind={BUTTON_KIND.secondary}
                icon={<ShareIcon />}
              />
            </StatefulTooltip>
          </Link>
        </div>
        <LabelMedium
          className={css({
            gridColumn: "1 / 2",
            gridRow: "2 / 3",
          })}
          color={COLORS.gray400}
        >
          Balance:
        </LabelMedium>
        <LabelMedium
          className={css({
            gridColumn: "2 / 4",
            gridRow: "2 / 3",
          })}
          color={COLORS.gray50}
        >
          {`${formatEther(balance)} NIL`}
        </LabelMedium>
        <LabelMedium
          className={css({
            gridColumn: "1 / 2",
            gridRow: "3 / 4",
          })}
          color={COLORS.gray400}
        >
          Tokens:
        </LabelMedium>
        <div
          className={css({
            gridColumn: "2 / 4",
            gridRow: "3 / 4",
          })}
        >
          {Object.entries(tokens).map(([token, amount]) => {
            return (
              <ParagraphMedium key={token}>
                <Link to={addressRoute} params={{ address: token }}>
                  {getTokenSymbolByAddress(token)}
                </Link>
                {": "}
                {amount.toString(10)}
              </ParagraphMedium>
            );
          })}
          {Object.keys(tokens).length === 0 && (
            <LabelMedium color={COLORS.gray400}>No tokens</LabelMedium>
          )}
        </div>
      </div>
      <div>
        {functions.map((func) => {
          return (
            <Method
              key={func.name}
              func={func}
              isOpen={activeKeys[func.name]}
              error={errors[func.name] || undefined}
              result={callResult[func.name]}
              loading={loading[func.name] || false}
              params={callParams[func.name]}
              txHash={txHashes[func.name] || undefined}
              paramsHandler={(p) => {
                setParams(p);
              }}
            />
          );
        })}
      </div>
    </>
  );
};
