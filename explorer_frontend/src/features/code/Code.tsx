import { BUTTON_KIND, BUTTON_SIZE, Button, Card, Spinner } from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import {
  $code,
  $error,
  $warnings,
  changeCode,
  clickOnContractsButton,
  clickOnLogButton,
  compile,
  compileCodeFx,
  fetchCodeSnippetFx,
} from "./model";
import "./init";
import { type Diagnostic, linter } from "@codemirror/lint";
import { Prec } from "@codemirror/state";
import { type EditorView, keymap } from "@codemirror/view";
import { useStyletron } from "baseui";
import { expandProperty } from "inline-style-expand-shorthand";
import { type ReactNode, useMemo } from "react";
import { fetchSolidityCompiler } from "../../services/compiler";
import { getMobileStyles } from "../../styleHelpers";
import { useMobile } from "../shared";
import { SolidityCodeField } from "../shared/components/SolidityCodeField";
import { CompileVersionButton } from "./code-toolbar/CompileVersionButton";
import { useCompileButton } from "./hooks/useCompileButton";

interface CodeProps {
  extraMobileButton?: ReactNode;
}

export const Code = ({ extraMobileButton }: CodeProps) => {
  const [isMobile] = useMobile();
  const [code, isDownloading, errors, fetchingCodeSnippet, compiling, warnings] = useUnit([
    $code,
    fetchSolidityCompiler.pending,
    $error,
    fetchCodeSnippetFx.pending,
    compileCodeFx.pending,
    $warnings,
  ]);
  const [css, theme] = useStyletron();
  const btnTextContent = useCompileButton();

  const preventNewlineOnCmdEnter = useMemo(
    () =>
      Prec.highest(
        keymap.of([
          {
            key: "Mod-Enter",
            run: () => true,
          },
        ]),
      ),
    [],
  );

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

    return [preventNewlineOnCmdEnter, linter(solidityLinter)];
  }, [errors, warnings, preventNewlineOnCmdEnter]);

  const noCode = code.trim().length === 0;
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
              height: `calc(100% - ${isMobile ? "32px - 8px - 8px - 48px - 8px - 48px - 8px" : "0px"})`,
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
                backgroundColor: `${theme.colors.backgroundPrimary} !important`,
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
            <CompileVersionButton
              isLoading={isDownloading || compiling}
              onClick={() => compile()}
              disabled={noCode}
              content={btnTextContent}
            />
            {isMobile && extraMobileButton && extraMobileButton}

            <Button
              overrides={{
                Root: {
                  style: {
                    gridColumn: "1 / 2",
                    backgroundColor: theme.colors.backgroundSecondary,
                    ":hover": {
                      backgroundColor: theme.colors.backgroundTertiary,
                    },
                  },
                },
              }}
              kind={BUTTON_KIND.secondary}
              size={BUTTON_SIZE.large}
              onClick={() => {
                clickOnLogButton();
              }}
            >
              Logs
            </Button>
            <Button
              overrides={{
                Root: {
                  style: {
                    gridColumn: "2 / 3",
                    backgroundColor: theme.colors.backgroundSecondary,
                    ":hover": {
                      backgroundColor: theme.colors.backgroundTertiary,
                    },
                  },
                },
              }}
              kind={BUTTON_KIND.secondary}
              size={BUTTON_SIZE.large}
              onClick={() => {
                clickOnContractsButton();
              }}
            >
              Contracts
            </Button>
          </div>
        )}
      </div>
    </Card>
  );
};
