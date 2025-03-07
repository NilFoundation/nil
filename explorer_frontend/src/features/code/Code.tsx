import { BUTTON_KIND, BUTTON_SIZE, Button, COLORS, Card, Spinner } from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import {
  $code,
  $error,
  $warnings,
  changeCode,
  compile,
  compileCodeFx,
  fetchCodeSnippetFx,
  isTutorialPage,
  сlickOnContractsButton,
  сlickOnLogButton,
} from "./model";
import "./init";
import { type Diagnostic, linter } from "@codemirror/lint";
import type { EditorView } from "@codemirror/view";
import { useStyletron } from "baseui";
import { expandProperty } from "inline-style-expand-shorthand";
import { type ReactNode, memo, useMemo } from "react";
import { fetchSolidityCompiler } from "../../services/compiler";
import { getMobileStyles } from "../../styleHelpers";
import { useMobile } from "../shared";
import { SolidityCodeField } from "../shared/components/SolidityCodeField";
import { CodeToolbar } from "./code-toolbar/CodeToolbar";
import { useCompileButton } from "./hooks/useCompileButton";

interface CodeProps {
  extraMobileTutorialButton?: ReactNode;
  runCheckButton?: ReactNode;
}

const MemoizedCodeToolbar = memo(CodeToolbar);

export const Code = ({
  extraMobileTutorialButton: extraMobileButtons,
  runCheckButton: runCheckButtons,
}: CodeProps) => {
  const [isMobile] = useMobile();
  const [code, isDownloading, errors, fetchingCodeSnippet, compiling, warnings, isTutorial] =
    useUnit([
      $code,
      fetchSolidityCompiler.pending,
      $error,
      fetchCodeSnippetFx.pending,
      compileCodeFx.pending,
      $warnings,
      isTutorialPage
    ]);
  const [css] = useStyletron();

  const codemirrorExtensions = useMemo(() => {
    const solidityLinter = (view: EditorView) => {
      const displayErrors: Diagnostic[] = errors.map((error) => {
        return {
          from: view.state.doc.line(error.line).from,
          to: view.state.doc.line(error.line).to,
          message: error.message,
          severity: "error",
        };
      });

      const displayWarnings: Diagnostic[] = warnings.map((warning) => {
        return {
          from: view.state.doc.line(warning.line).from,
          to: view.state.doc.line(warning.line).to,
          message: warning.message,
          severity: "warning",
        };
      });

      return [...displayErrors, ...displayWarnings];
    };

    return [linter(solidityLinter)];
  }, [errors, warnings]);

  const noCode = code.trim().length === 0;
  const btnContent = useCompileButton();
  return (
    <Card
      overrides={{
        Root: {
          style: {
            backgroundColor: "transparent",
            width: "100%",
            maxWidth: "none",
            ...expandProperty("padding", "0"),
            height: "100%",
            ...getMobileStyles({
              width: "calc(100vw - 32px)",
              height: "auto",
            }),
          },
        },
        Body: {
          style: {
            display: "flex",
            flexDirection: "column",
            position: "relative",
            height: "100%",
            marginBottom: 0,
            paddingBottom: "16px",
            ...getMobileStyles({
              gap: "8px",
            }),
          },
        },
        Contents: {
          style: {
            height: "100%",
          },
        },
      }}
    >
      <div
        className={css({
          flexBasis: "100%",
          height: "100%",
        })}
      >
        <div
          className={css({
            display: "flex",
            justifyContent: "flex-start",
            gap: "8px",
            paddingBottom: "8px",
            ...getMobileStyles({
              flexDirection: "column",
              gap: "8px",
            }),
            zIndex: 2,
            height: "auto",
          })}
        >
          <MemoizedCodeToolbar disabled={isDownloading} />
          {!isMobile && (
            <Button
              kind={BUTTON_KIND.primary}
              isLoading={isDownloading || compiling}
              size={BUTTON_SIZE.default}
              onClick={() => compile()}
              disabled={noCode}
              overrides={{
                Root: {
                  style: {
                    whiteSpace: "nowrap",
                    lineHeight: 1,
                    marginLeft: "auto",

                  },
                },
              }}
              data-testid="compile-button"
            >
              {btnContent}
            </Button>
          )}
          {!isMobile && runCheckButtons && runCheckButtons}
        </div>
        {fetchingCodeSnippet ? (
          <div
            className={css({
              display: "flex",
              justifyContent: "center",
              alignItems: "center",
              width: "100%",
              height: "100%",
              borderTopLeftRadius: "12px",
              borderTopRightRadius: "12px",
              borderBottomLeftRadius: "12px",
              borderBottomRightRadius: "12px",
            })}
          >
            <Spinner />
          </div>
        ) : (
          <div
            className={css({
              width: "100%",
              height: `calc(100% - ${isMobile ? "32px - 8px - 8px - 48px - 8px - 48px - 8px" : "48px - 8px"})`,
              borderTopLeftRadius: "12px",
              borderTopRightRadius: "12px",
              borderBottomLeftRadius: "12px",
              borderBottomRightRadius: "12px",
            })}
          >
            <SolidityCodeField
              extensions={codemirrorExtensions}
              editable
              readOnly={false}
              code={code}
              onChange={(text) => {
                changeCode(`${text}`);
              }}
              className={css({
                paddingBottom: "0!important",
                height: "100%",
                overflow: "auto!important",
                overscrollBehavior: "contain",
                backgroundColor: isTutorial ? `${COLORS.blue900} !important` : COLORS.gray900,
              })}
              data-testid="code-field"
            />
          </div>
        )}
        {isMobile && (
          <div
            className={css({
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gridTemplateRows: "48px 48px",
              gap: "8px",
              paddingTop: "8px",
            })}
          >
            <Button
              kind={BUTTON_KIND.primary}
              isLoading={isDownloading || compiling}
              onClick={() => compile()}
              disabled={noCode}
              overrides={{
                Root: {
                  style: {
                    lineHeight: 1,
                    gridColumn: "1 / 3",

                  },
                },
              }}
              data-testid="compile-button"
            >
              {btnContent}
            </Button>
            <Button
              overrides={{
                Root: {
                  style: {
                    gridColumn: "1 / 2",
                    ...(isTutorial ? {
                      backgroundColor: COLORS.blue800,
                      ':hover': {
                        backgroundColor: COLORS.blue700,
                      }
                    } : {}
                    )
                  },
                },
              }}
              kind={BUTTON_KIND.secondary}
              size={BUTTON_SIZE.large}
              onClick={() => {
                сlickOnLogButton();
              }}
            >
              Logs
            </Button>
            <Button
              overrides={{
                Root: {
                  style: {
                    gridColumn: "2 / 3",
                    ...(isTutorial ? {
                      backgroundColor: COLORS.blue800,
                      ':hover': {
                        backgroundColor: COLORS.blue700,
                      }
                    } : {}
                    )
                  },
                },
              }}
              kind={BUTTON_KIND.secondary}
              size={BUTTON_SIZE.large}
              onClick={() => {
                сlickOnContractsButton();
              }}
            >
              Contracts
            </Button>
            {isMobile && extraMobileButtons && extraMobileButtons}
            {isMobile && runCheckButtons && runCheckButtons}
          </div>
        )}
      </div>
    </Card>
  );
};
