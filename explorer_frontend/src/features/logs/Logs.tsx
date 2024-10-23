import { useUnit } from "effector-react";
import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  Card,
  LabelMedium,
  SPACE,
} from "@nilfoundation/ui-kit";
import "./init";
import { $logs } from "./model";
import { useStyletron } from "baseui";
import { LogsGreeting } from "./LogsGreeting";
import { useMobile } from "../shared";
import { getMobileStyles } from "../../styleHelpers";
import { LayoutComponent, setActiveComponent } from "../../pages/sandbox/model";
import { useCallback, useEffect, useRef } from "react";

export const Logs = () => {
  const [logs] = useUnit([$logs]);
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
          <LabelMedium color={COLORS.gray50}>Logs</LabelMedium>
        </div>
      )}
      <Card
        overrides={{
          Root: {
            style: {
              backgroundColor: "#212121",
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
      </Card>
    </div>
  );
};
