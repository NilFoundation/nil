import {
  BUTTON_KIND,
  Button,
  COLORS,
  CopyButton,
  Input,
  LabelLarge,
  LabelMedium,
  LabelSmall,
  NOTIFICATION_KIND,
  Notification,
  PlusIcon,
  StatefulTooltip,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { ButtonOverrides } from "baseui/button";
import { useUnit } from "effector-react";
import { useEffect, useState } from "react";
import { formatEther } from "viem";
import { $rpcIsHealthy } from "../../healthcheck/model";
import { OverflowEllipsis, useMobile } from "../../shared";
import { ActiveComponent } from "../ActiveComponent";
import CheckmarkIcon from "../assets/checkmark.svg";
import Linkicon from "../assets/link.svg";
import {
  $balance,
  $balanceToken,
  $latestActivity,
  $rpcUrl,
  $smartAccount,
  clearLatestActivity,
  createSmartAccountFx,
  setActiveComponent,
  topUpSmartAccountBalanceFx,
} from "../model";
import { Token } from "./Token";
import { styles } from "./styles";

const btnOverrides: ButtonOverrides = {
  Root: {
    style: ({ $disabled }) => ({
      backgroundColor: $disabled ? `${COLORS.gray400}!important` : "",
      width: "100%",
    }),
  },
};

const MainScreen = () => {
  const [css, theme] = useStyletron();
  const [copied, setCopied] = useState(false);
  const rpcUrl = useUnit($rpcUrl);
  const latestActivity = useUnit($latestActivity);
  const [smartAccount, balance, balanceToken, isPendingSmartAccountCreation, rpcIsHealthy] =
    useUnit([$smartAccount, $balance, $balanceToken, createSmartAccountFx.pending, $rpcIsHealthy]);
  const [isPendingTopUp] = useUnit([topUpSmartAccountBalanceFx.pending]);
  const displayBalance = balance === null ? "-" : formatEther(balance);
  const [isMobile] = useMobile();
  const formattedBalance =
    isMobile && displayBalance !== "-" ? displayBalance.slice(0, 9) : displayBalance;
  const address = smartAccount ? smartAccount.address : null;

  useEffect(() => {
    if (latestActivity) {
      console.log("New activity detected:", latestActivity);

      // Auto-hide the notification after 15 seconds
      const timeoutId = setTimeout(() => {
        clearLatestActivity();
      }, 15000);

      return () => clearTimeout(timeoutId);
    }
  }, [latestActivity]);

  const handleNavigate = () => {
    const endpointUrl = window.location.origin;
    if (endpointUrl) {
      window.open(`${endpointUrl}/tx/${latestActivity.txHash}`, "_blank");
    } else {
      console.error("VITE_GET_ENDPOINT_URL is not set.");
    }
  };

  // Handle copy functionality
  const handleCopy = async () => {
    if (rpcUrl && typeof rpcUrl === "string") {
      await navigator.clipboard.writeText(rpcUrl);

      setCopied(true);

      // Reset text after 2 seconds
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: "24px",
        maxHeight: "531px",
        overflowY: "auto",
        overscrollBehavior: "contain",
        "@media (max-width: 419px)": {
          width: "calc(100vw - 48px)",
        },
      })}
    >
      <div
        className={css({
          width: "100%",
          display: "flex",
          flexDirection: "column",
          position: "sticky",
          alignItems: "center",
          gap: "24px",
          top: 0,
          backgroundColor: theme.colors.backgroundSecondary,
        })}
      >
        <LabelLarge>Smart Account</LabelLarge>
        {address !== null && (
          <div
            className={css({
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: "1ch",
            })}
          >
            <LabelMedium width="250px" color={COLORS.gray200}>
              <OverflowEllipsis>{address}</OverflowEllipsis>
            </LabelMedium>
            <CopyButton textToCopy={address} disabled={address === null} color={COLORS.gray200} />
          </div>
        )}
        {rpcUrl !== null && (
          <div
            className={css({
              display: "flex",
              alignItems: "center",
              width: "100%",
              gap: "12px",
            })}
          >
            {/* Read-Only Input */}
            <Input
              placeholder="Enter your RPC URL"
              value={rpcUrl}
              readOnly
              overrides={{
                Root: {
                  style: {
                    flex: 1,
                    height: "48px",
                    backgroundColor: theme.colors.rpcUrlBackgroundColor,
                    ":hover": {
                      backgroundColor: theme.colors.rpcUrlBackgroundHoverColor,
                    },
                    boxShadow: "none",
                  },
                },
              }}
            />
            {/* Copy Button */}
            <Button
              kind={BUTTON_KIND.secondary}
              onClick={handleCopy}
              overrides={{
                Root: {
                  style: {
                    width: "120px",
                    padding: "0 12px",
                    height: "48px",
                    backgroundColor: COLORS.gray50,
                    color: COLORS.gray800,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    gap: "8px",
                    ":hover": {
                      backgroundColor: COLORS.gray100,
                      color: COLORS.gray800,
                    },
                  },
                },
              }}
            >
              {copied ? <img src={CheckmarkIcon} alt="Copied" width={20} height={20} /> : null}
              {copied ? "Copied!" : "Copy"}
            </Button>
          </div>
        )}
      </div>

      <ul
        className={css({
          width: "100%",
          maxWidth: "100vw",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          overflowX: "auto",
          overflowY: "auto",
          maxHeight: "250px",
          scrollbarWidth: "none",
          "-ms-overflow-style": "none",
          "&::-webkit-scrollbar": { display: "none" },
        })}
      >
        {displayBalance !== "-" && (
          <li className={css(styles.menuItem)}>
            <Token balance={formattedBalance} name="NIL" isMain />
          </li>
        )}

        {balanceToken !== null &&
          Object.keys(balanceToken).length !== 0 &&
          Object.entries(balanceToken).map(([name, balance]) => (
            <>
              <li key={`divider-${name}`} className={css(styles.divider)} />
              <li key={name} className={css(styles.menuItem)}>
                <Token balance={balance.toString()} name={name} isMain={false} />
              </li>
            </>
          ))}
      </ul>

      {latestActivity != null && (
        <div
          style={{
            position: "fixed",
            bottom: "70px",
            left: 0,
            right: 0,
            zIndex: 1000,
            display: "flex",
            justifyContent: "center",
            boxSizing: "border-box",
          }}
        >
          <Notification
            closeable={true}
            kind={
              latestActivity.successful ? NOTIFICATION_KIND.positive : NOTIFICATION_KIND.negative
            }
            hideIcon={true}
            icon={
              <StatefulTooltip
                content="Open in Explorer"
                showArrow={true}
                placement="top"
                overrides={{
                  Body: { style: { zIndex: 3000 } },
                  Inner: { style: { backgroundColor: COLORS.gray50 } },
                }}
              >
                <div
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    padding: "4px",
                    cursor: "pointer",
                  }}
                  onClick={handleNavigate}
                >
                  <img src={Linkicon} alt="Link" width={24} height={24} />
                </div>
              </StatefulTooltip>
            }
            overrides={{
              Body: {
                style: {
                  backgroundColor: COLORS.gray700,
                  color: COLORS.gray200,
                  width: "100%",
                  maxWidth: "calc(100% - 48px)",
                  padding: "12px 16px",
                  borderRadius: "8px",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  fontWeight: "bold",
                  boxSizing: "border-box",
                },
              },
            }}
          >
            {/* Notification Content */}
            <div>
              <LabelSmall
                style={{ color: latestActivity.successful ? COLORS.green500 : COLORS.red500 }}
                onClick={handleNavigate}
              >
                {latestActivity.successful ? "Topped Up Successfully" : "Top-Up Failed"}
              </LabelSmall>
              <LabelSmall
                style={{
                  color: COLORS.gray200,
                  cursor: "pointer",
                  transition: "color 0.2s ease-in-out",
                  ":hover": { color: COLORS.gray300 },
                }}
                onClick={handleNavigate}
              >
                {`${latestActivity.txHash.slice(0, 6)}...${latestActivity.txHash.slice(-4)}`}
              </LabelSmall>
            </div>
          </Notification>
        </div>
      )}
      <Button
        kind={BUTTON_KIND.primary}
        onClick={() => setActiveComponent(ActiveComponent.Topup)}
        isLoading={isPendingTopUp}
        disabled={isPendingTopUp || isPendingSmartAccountCreation || !smartAccount || !rpcIsHealthy}
        overrides={btnOverrides}
        className={css({
          whiteSpace: "nowrap",
        })}
      >
        <PlusIcon size={24} />
        Top up
      </Button>
    </div>
  );
};

export { MainScreen };
