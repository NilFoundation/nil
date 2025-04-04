import { BUTTON_KIND, BUTTON_SIZE, Button, Card, Spinner } from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import {
  $code,
  $codeError,
  $codeWarnings,
  $script,
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
import { type Diagnostic, linter } from "@codemirror/lint";
import { Prec } from "@codemirror/state";
import { type EditorView, keymap } from "@codemirror/view";
import { useStyletron } from "baseui";
import { expandProperty } from "inline-style-expand-shorthand";
import { type ReactNode, memo, useMemo } from "react";
import { fetchSolidityCompiler } from "../../services/compiler";
import { getMobileStyles } from "../../styleHelpers";
import { useMobile } from "../shared";
import { CodeToolbar } from "./code-toolbar/CodeToolbar";
import { useCompileButton, useRunScriptButton } from "./hooks/useCompileButton";
import { basicSetup } from "@uiw/react-codemirror";
import { solidity } from "@replit/codemirror-lang-solidity";
import { javascriptLanguage } from "@codemirror/lang-javascript";
import { CustomCodeField } from "../shared/components/CustomCodeField";
interface CodeProps {
  extraMobileButton?: ReactNode;
  extraToolbarButton?: ReactNode;
  isSolidity?: boolean;
}

const MemoizedCodeToolbar = memo(CodeToolbar);

export const Code = ({ extraMobileButton, extraToolbarButton, isSolidity }: CodeProps) => {
  const [isMobile] = useMobile();
  const [code, script, isDownloading, errors, fetchingCodeSnippet, compiling, warnings] = useUnit([
    $code,
    $script,
    fetchSolidityCompiler.pending,
    $codeError,
    fetchProjectFx.pending,
    compileCodeFx.pending,
    $codeWarnings,
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
  }, [errors, warnings, preventNewlineOnCmdEnter, isMobile, isSolidity]);

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
          <MemoizedCodeToolbar
            disabled={isDownloading}
            extraToolbarButton={extraToolbarButton}
            isSolidity={isSolidity}
          />
          {!isMobile && (
            <Button
              kind={BUTTON_KIND.primary}
              isLoading={isDownloading || compiling}
              size={BUTTON_SIZE.default}
              onClick={() => btnClickEvent()}
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
              data-testid="compile-run-button"
            >
              {btnTextContent}
            </Button>
          )}
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
              onClick={() => btnClickEvent()}
              disabled={noCode}
              overrides={{
                Root: {
                  style: {
                    lineHeight: 1,
                    gridColumn: "1 / 3",
                  },
                },
              }}
              data-testid="compile-run-button"
            >
              {btnTextContent}
            </Button>
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
            {isMobile && extraMobileButton && extraMobileButton}
            {isMobile && extraToolbarButton && extraToolbarButton}
          </div>
        )}
      </div>
    </Card>
  );
};
