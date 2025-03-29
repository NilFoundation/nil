import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  COLORS,
  Card,
  CodeField,
  HeadingLarge,
  HeadingMedium,
  LabelMedium,
  ListItem,
  ParagraphMedium,
  SPACE,
} from "@nilfoundation/ui-kit";
import { Link } from "atomic-router-react";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import Markdown from "react-markdown";
import {
  $tutorialChecksState,
  TutorialChecksStatus,
  clickOnTutorialsBackButton,
} from "../../pages/tutorials/model";
import { getMobileStyles } from "../../styleHelpers";
import { clickOnBackButton } from "../code/model";
import { useMobile } from "../shared";
import { Divider } from "../shared/components/Divider";
import { linkStyles } from "../shared/components/Link";
import { runTutorialCheck, runTutorialCheckFx } from "../tutorial-check/model";
import { $tutorial } from "./model";

export const TutorialText = () => {
  const [isMobile] = useMobile();
  const [tutorial, runningChecks, tutorialChecks] = useUnit([
    $tutorial,
    runTutorialCheckFx.pending,
    $tutorialChecksState,
  ]);

  const [css, theme] = useStyletron();
  const HeaderOne = (props: any) => (
    <HeadingLarge
      {...props}
      className={css({
        fontSize: "32px",
        fontWeight: 500,
        marginBottom: "12px",
        lineHeight: "40px",
        marginTop: "10px",
      })}
    />
  );

  const HeaderTwo = (props: any) => (
    <HeadingMedium
      {...props}
      className={css({
        fontSize: "24px",
        fontWeight: 400,
        marginBottom: "8px",
        lineHeight: "30px",
        marginTop: "10px",
      })}
    />
  );

  const CustomParagraph = (props: any) => (
    <ParagraphMedium
      {...props}
      className={css({
        lineHeight: "24px",
        marginBottom: "6px",
        marginTop: "12px",
      })}
    />
  );

  const CustomItalics = (props: any) => (
    <span {...props} className={css({ fontStyle: "italic" })} />
  );

  const CustomListItem = (props: any) => (
    <ListItem
      {...props}
      overrides={{
        Content: {
          style: {
            display: "inline",
            backgroundColor: COLORS.blue900,
          },
        },
      }}
    />
  );

  const CustomLink = (props: any) => <Link {...props} to={props.href} style={linkStyles.link} />;
  const CustomCodeField = ({ node, inline, className, children, ...props }: any) => {
    const codeContent = Array.isArray(children) ? children.join("") : children;
    return (
      <CodeField
        className={css({
          backgroundColor: `${COLORS.blue800} !important`,
        })}
        code={codeContent}
        themeOverrides={{
          settings: {
            background: COLORS.blue800,
          },
        }}
        {...props}
      />
    );
  };

  let checkButtonBckgColor: string;
  switch (tutorialChecks) {
    case TutorialChecksStatus.Successful:
      checkButtonBckgColor = COLORS.green200;
      break;
    case TutorialChecksStatus.Failed:
      checkButtonBckgColor = COLORS.red200;
      break;
    case TutorialChecksStatus.Initialized:
      checkButtonBckgColor = COLORS.yellow200;
      break;
    default:
      checkButtonBckgColor = COLORS.black;
      break;
  }

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        height: "100%",
        position: "relative",
        ...getMobileStyles({
          height: "calc(100vh - 109px)",
        }),
      })}
    >
      {!isMobile && (
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
            onClick={() => clickOnTutorialsBackButton()}
          >
            <ArrowUpIcon
              size={12}
              className={css({
                transform: "rotate(-90deg)",
              })}
            />
          </Button>
          <LabelMedium color={COLORS.gray50}>Tutorials</LabelMedium>
        </div>
      )}
      <Card
        overrides={{
          Root: {
            style: {
              maxWidth: isMobile ? "calc(100vw - 20px)" : "none",
              width: isMobile ? "100%" : "none",
              height: "100%",
              backgroundColor: COLORS.blue900,
              paddingRight: "0",
              paddingLeft: "0",
              flexDirection: "column",
              paddingBottom: "24px",
              display: "flex",
              flexGrow: 1,
              overflow: "hidden",
            },
          },
          Contents: {
            style: {
              maxWidth: "none",
              width: "100%",
              paddingBottom: "24px",
              height: "100%",
              display: "flex",
              flexDirection: "column",
              padding: "8px",
              overflow: "hidden",
              ...getMobileStyles({
                height: "calc(100vh - 154px)",
              }),
            },
          },
          Body: {
            style: {
              height: "auto",
              width: "100%",
              maxWidth: "none",
              overflowY: "auto",
              flexGrow: 1,
              display: "flex",
              flexDirection: "column",
            },
          },
        }}
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
            <LabelMedium color={COLORS.gray50}>Tutorials</LabelMedium>
          </div>
        )}
        <div
          className={css({
            flexGrow: 1,
            overflowY: "auto",
            paddingRight: "8px",
          })}
        >
          <Markdown
            components={{
              h1: HeaderOne,
              h2: HeaderTwo,
              p: CustomParagraph,
              li: CustomListItem,
              code: CustomCodeField,
              em: CustomItalics,
              a: CustomLink,
            }}
          >
            {tutorial.text}
          </Markdown>
          <div
            className={css({
              marginTop: "18px",
            })}
          />
          <Divider />
          <div
            className={css({
              display: "flex",
              flexDirection: "row",
              alignItems: "end",
            })}
          >
            <Button
              kind={BUTTON_KIND.secondary}
              isLoading={runningChecks}
              size={BUTTON_SIZE.default}
              onClick={() => runTutorialCheck()}
              disabled={tutorialChecks === TutorialChecksStatus.NotInitialized}
              overrides={{
                Root: {
                  style: {
                    whiteSpace: "nowrap",
                    lineHeight: 1,
                    marginLeft: "auto",
                    backgroundColor: checkButtonBckgColor,
                    color: COLORS.black,
                    marginTop: "18px",
                  },
                },
              }}
              data-testid="run-checks-button"
            >
              Run Checks
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
};
