import { BUTTON_KIND, BUTTON_SIZE, Button, Card, Spinner } from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import {
  $code,
  $codeError,
  $codeWarnings,
  $script,
  $scriptErrors,
  changeCode,
  changeScript,
  clickOnContractsButton,
  clickOnLogButton,
  compileCode,
  compileCodeFx,
  fetchProjectFx,
  runScript,
} from "./model";
import "./init";
import { javascriptLanguage } from "@codemirror/lang-javascript";
import { type Diagnostic, linter } from "@codemirror/lint";
import { Prec } from "@codemirror/state";
import { type EditorView, keymap } from "@codemirror/view";
import { solidity } from "@replit/codemirror-lang-solidity";
import { basicSetup } from "@uiw/react-codemirror";
import { useStyletron } from "baseui";
import { expandProperty } from "inline-style-expand-shorthand";
import { type ReactNode, useMemo } from "react";
import { fetchSolidityCompiler } from "../../services/compiler";
import { getMobileStyles } from "../../styleHelpers";
import { useMobile } from "../shared";
import { CustomCodeField } from "../shared/components/CustomCodeField";
import { CompileVersionButton } from "./code-toolbar/CompileVersionButton";
import { useCompileButton, useRunScriptButton } from "./hooks/useCompileButton";
interface CodeProps {
  extraMobileButton?: ReactNode;
  isSolidity?: boolean;
}

export const Code = ({ extraMobileButton, isSolidity }: CodeProps) => {
  const [isMobile] = useMobile();
  const [
    code,
    script,
    isDownloading,
    codeErrors,
    fetchingCodeSnippet,
    compiling,
    warnings,
    scriptErrors,
  ] = useUnit([
    $code,
    $script,
    fetchSolidityCompiler.pending,
    $codeError,
    fetchProjectFx.pending,
    compileCodeFx.pending,
    $codeWarnings,
    $scriptErrors,
  ]);
  const [css, theme] = useStyletron();
  const btnTextContent = isSolidity ? useCompileButton() : useRunScriptButton();

  const changeEvent = isSolidity ? changeCode : changeScript;
  const btnClickEvent = isSolidity ? compileCode : runScript;

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
    const codeLinter = (view: EditorView) => {
      if (isSolidity) {
        const displayErrors: Diagnostic[] = codeErrors.map((error) => {
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
      }

      const displayErrors: Diagnostic[] = scriptErrors.map((error) => {
        return {
          from: view.state.doc.line(error.line).from,
          to: view.state.doc.line(error.line).to,
          message: error.message,
          severity: "error",
        };
      });

      return [...displayErrors];
    };

    const lang = isSolidity
      ? solidity
      : javascriptLanguage.configure({ dialect: "ts" }, "typescript");

    return [
      preventNewlineOnCmdEnter,
      lang,
      ...basicSetup({
        lineNumbers: !isMobile,
      }),
      linter(codeLinter),
    ];
  }, [codeErrors, scriptErrors, warnings, preventNewlineOnCmdEnter, isMobile, isSolidity]);

  const noCode = code.trim().length === 0;
  const resCode = isSolidity ? code : script;

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
            ...expandProperty("borderRadius", "16px"),
            ...getMobileStyles({
              width: "calc(100vw - 32px)",
              ...expandProperty("borderRadius", "0"),
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
              ...expandProperty("borderRadius", "16px"),
            })}
          >
            <Spinner />
          </div>
        ) : (
          <div
            className={css({
              width: "100%",
              height: `calc(100% - ${isMobile ? "32px - 8px - 8px - 48px - 8px - 8px" : "0px"})`,
              ...expandProperty("borderRadius", "16px"),
              overflow: "auto !important",
            })}
          >
            <CustomCodeField
              extensions={codemirrorExtensions}
              code={resCode}
              onChange={(text) => {
                changeEvent(`${text}`);
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
              position: "sticky",
              bottom: "0",
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gridTemplateRows: "48px 48px",
              gap: "8px",
              paddingTop: "8px",
            })}
          >
            <CompileVersionButton
              isLoading={isDownloading || compiling}
              onClick={() => btnClickEvent()}
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
