import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  ButtonIcon,
  COLORS,
  Card,
  LabelMedium,
  SPACE,
  StatefulTooltip,
} from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import "../init";
import { useStyletron } from "baseui";
import { useCallback, useEffect, useRef } from "react";
import { getMobileStyles } from "../../../styleHelpers";
import { clickOnBackButton } from "../../code/model";
import { ClearIcon, useMobile } from "../../shared";
import { $logs, clearLogs } from "../model";
import { LogsGreeting } from "./LogsGreeting";

export const Logs = () => {
  const [logs] = useUnit([$logs]);
  const [css, theme] = useStyletron();
  const [isMobile] = useMobile();
  const lastItemRef = useRef<HTMLDivElement>(null);
  const scrollToBottom = useCallback(() => {
    lastItemRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: <explanation>
  useEffect(() => {
    scrollToBottom();
  }, [logs, scrollToBottom]);

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        height: "100%",
        ...getMobileStyles({
          height: "calc(100vh - 109px)",
        }),
      })}
      data-testid="playground-logs"
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
                  backgroundColor: theme.colors.backgroundSecondary,
                  ":hover": {
                    backgroundColor: theme.colors.backgroundTertiary,
                  },
                },
              },
            }}
            kind={BUTTON_KIND.secondary}
            size={BUTTON_SIZE.compact}
            onClick={() => clickOnBackButton()}
          >
            <ArrowUpIcon
              size={12}
              className={css({
                transform: "rotate(-90deg)",
              })}
            />
          </Button>
          <LabelMedium color={COLORS.gray50}>Logs</LabelMedium>
        </div>
      )}
      <Card
        overrides={{
          Root: {
            style: {
              backgroundColor: theme.colors.backgroundPrimary,
              width: "100%",
              maxWidth: "none",
              height: "100%",
              paddingRight: 0,
              paddingLeft: 0,
              paddingTop: 0,
            },
          },
          Contents: {
            style: {
              display: "flex",
              flexDirection: "column",
              gap: SPACE[8],
              height: "100%",
              position: "relative",
            },
          },
          Body: {
            style: {
              overflow: "auto",
              flex: "1 0",
              overscrollBehavior: "contain",
              paddingRight: "16px",
              paddingLeft: "16px",
              paddingTop: "16px",
              marginBottom: 0,
            },
          },
        }}
      >
        <LogsGreeting
          className={css({
            marginBottom: SPACE[32],
          })}
        />
        {logs.map(({ payload, id, shortDescription }) => {
          return (
            <div
              key={id}
              className={css({
                paddingBottom: SPACE[8],
                marginBottom: SPACE[16],
              })}
            >
              <div
                className={css({
                  display: "flex",
                  flexDirection: "row",
                  justifyContent: "flex-start",
                  alignItems: "center",
                })}
              >
                {shortDescription}
              </div>
              <div
                className={css({
                  paddingTop: SPACE[8],
                  whiteSpace: "pre-line",
                })}
              >
                {payload}
              </div>
            </div>
          );
        })}
        <div
          className={css({
            backgroundColor: "transparent",
            height: "1px",
            zIndex: -1,
          })}
          ref={lastItemRef}
        />
        <StatefulTooltip content="Clear all" showArrow={false} placement="bottom" popoverMargin={4}>
          <ButtonIcon
            kind={BUTTON_KIND.secondary}
            icon={<ClearIcon />}
            onClick={() => clearLogs()}
            overrides={{
              Root: {
                style: {
                  position: "absolute",
                  top: "16px",
                  right: "16px",
                  backgroundColor: theme.colors.inputButtonAndDropdownOverrideBackgroundColor,
                  ":hover": {
                    backgroundColor:
                      theme.colors.inputButtonAndDropdownOverrideBackgroundHoverColor,
                  },
                },
              },
            }}
            data-testid="clear-logs"
          />
        </StatefulTooltip>
      </Card>
    </div>
  );
};
