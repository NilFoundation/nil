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
import { isTutorialPage, сlickOnBackButton } from "../../code/model";
import { ClearIcon, useMobile } from "../../shared";
import { $logs, clearLogs } from "../model";
import { LogsGreeting } from "./LogsGreeting";

export const Logs = () => {
  const [logs, isTutorial] = useUnit([$logs, isTutorialPage]);
  const [css] = useStyletron();
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
                  backgroundColor: isTutorial ? COLORS.blue800 : COLORS.gray800,
                  ":hover": {
                    backgroundColor: isTutorial ? COLORS.blue700 : COLORS.gray700,
                  }
                },
              },
            }}
            kind={BUTTON_KIND.secondary}
            size={BUTTON_SIZE.compact}
            onClick={() => сlickOnBackButton()}
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
              backgroundColor: (isTutorial ? COLORS.blue900 : COLORS.gray900),
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
                  backgroundColor: (isTutorial ? COLORS.blue800 : COLORS.gray800),
                  ':hover': {
                    backgroundColor: (isTutorial ? COLORS.blue700 : COLORS.gray700),
                  }
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
