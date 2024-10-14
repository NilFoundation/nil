import { useUnit } from "effector-react";
import { collapseLog, expandLog } from "./model";
import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  Card,
  ChevronDownIcon,
  ChevronUpIcon,
  LabelMedium,
  MonoHeadingMedium,
  MonoParagraphMedium,
  SPACE,
} from "@nilfoundation/ui-kit";
import "./init";
import { $logsWithOpen } from "./init";
import { useStyletron } from "baseui";
import { LogsGreeting } from "./LogsGreeting";
import { useMobile } from "../shared";
import { getMobileStyles } from "../../styleHelpers";
import { LayoutComponent, setActiveComponent } from "../../pages/sandbox/model";

export const Logs = () => {
  const [logs] = useUnit([$logsWithOpen]);
  const [css] = useStyletron();
  const [isMobile] = useMobile();

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
            },
          },
        }}
      >
        <LogsGreeting
          className={css({
            marginBottom: SPACE[24],
          })}
        />
        {logs.map((log, index) => {
          return (
            <div
              key={log.id}
              className={css({
                paddingBottom: SPACE[8],
                borderBottom: index === logs.length - 1 ? "none" : `1px solid ${COLORS.gray800}`,
                marginBottom: SPACE[8],
              })}
            >
              <div
                className={css({
                  cursor: "pointer",
                  marginBottom: log.isOpen ? SPACE[8] : 0,
                  display: "flex",
                  flexDirection: "row",
                  justifyContent: "flex-start",
                  alignItems: "center",
                })}
                onMouseDown={(e) => {
                  e.preventDefault();
                  log.isOpen ? collapseLog(log.id) : expandLog(log.id);
                }}
                onTouchStart={(e) => {
                  e.preventDefault();
                  log.isOpen ? collapseLog(log.id) : expandLog(log.id);
                }}
              >
                <MonoHeadingMedium color={COLORS.gray400}>{log.shortDescription}</MonoHeadingMedium>
                <Button
                  kind={BUTTON_KIND.secondary}
                  size={BUTTON_SIZE.compact}
                  className={css({
                    marginLeft: SPACE[8],
                  })}
                >
                  {log.isOpen ? <ChevronUpIcon color={COLORS.gray400} /> : <ChevronDownIcon color={COLORS.gray400} />}
                </Button>
              </div>
              {log.isOpen && (
                <Card
                  overrides={{
                    Root: {
                      style: {
                        wordBreak: "break-all",
                        width: "100%",
                        maxWidth: "none",
                      },
                    },
                  }}
                >
                  <MonoParagraphMedium color={COLORS.gray200}>
                    {JSON.stringify(
                      log.payload,
                      (_, value) => (typeof value === "bigint" ? `${value.toString(10)}n` : value),
                      2,
                    )}
                  </MonoParagraphMedium>
                </Card>
              )}
            </div>
          );
        })}
      </Card>
    </div>
  );
};
