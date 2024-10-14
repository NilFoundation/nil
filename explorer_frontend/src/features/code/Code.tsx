import { useUnit } from "effector-react";
import { $code, $error, changeCode, compile, fetchCodeSnippetFx } from "./model";
import { BUTTON_KIND, BUTTON_SIZE, Button, COLORS, Card, CodeField, Spinner } from "@nilfoundation/ui-kit";
import "./init";
import { useStyletron } from "baseui";
import { solidity } from "@replit/codemirror-lang-solidity";
import { basicSetup } from "@uiw/react-codemirror";
import { memo, useMemo } from "react";
import { fetchSolidityCompiler } from "../../services/compiler";
import { linter, type Diagnostic } from "@codemirror/lint";
import { ShareCodePanel } from "./ShareCodePanel";
import { expandProperty } from "inline-style-expand-shorthand";
import { getMobileStyles } from "../../styleHelpers";
import { useMobile } from "../shared";
import { LayoutComponent, setActiveComponent } from "../../pages/sandbox/model";
import type { EditorView } from "@codemirror/view";
import type { Extension } from "@codemirror/state";

const MemoizedShareCodePanel = memo(ShareCodePanel);

export const Code = () => {
  const [isMobile] = useMobile();
  const [code, isDownloading, errors, fetchingCodeSnippet] = useUnit([
    $code,
    fetchSolidityCompiler.pending,
    $error,
    fetchCodeSnippetFx.pending,
  ]);
  const [css] = useStyletron();

  const codemirrorExtensions = useMemo<Extension[]>(() => {
    const solidityLinter = (view: EditorView) => {
      const diagnostics: Diagnostic[] = errors.map((error) => {
        return {
          from: view.state.doc.line(error.line).from,
          to: view.state.doc.line(error.line).to,
          message: error.message,
          severity: "error",
        };
      });
      return diagnostics;
    };
    return [solidity, ...basicSetup({
      lineNumbers: !isMobile,
    }), linter(solidityLinter)];
  }, [errors]);

  const noCode = code.trim().length === 0;

  return (
    <Card
      overrides={{
        Root: {
          style: {
            backgroundColor: COLORS.gray900,
            width: "100%",
            maxWidth: "none",
            ...expandProperty("padding", "0"),
            height: "100%",
            ...getMobileStyles({
              width: "calc(100vw - 32px)",
              height: "calc(100vh - 109px)",
              overflow: "scroll",
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
            paddinBottom: "16px",
            overflow: "scroll",
            overscrollBehavior: "contain",
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
        })}
      >
        {fetchingCodeSnippet ? (
          <Spinner />
        ) : (
          <CodeField
            extensions={codemirrorExtensions}
            editable
            readOnly={false}
            code={code}
            onChange={(text) => {
              changeCode(`${text}`);
            }}
            displayCopy={false}
            highlightOnHover={false}
            className={css({
              paddingBottom: "0!important",
            })}
            showLineNumbers={false}
          />
        )}
      </div>
      <div
        className={css({
          display: "flex",
          gap: "16px",
          position: "sticky",
          bottom: "-1px",
          paddingBottom: "24px",
          paddingTop: "24px",
          background: COLORS.gray900,
          ...getMobileStyles({
            flexDirection: "column",
            gap: "8px",
          }),
        })}
      >
        <Button
          kind={BUTTON_KIND.primary}
          isLoading={isDownloading}
          size={isMobile ? BUTTON_SIZE.large : BUTTON_SIZE.compact}
          onClick={() => compile()}
          disabled={noCode}
          overrides={{
            Root: {
              style: {
                marginLeft: "24px",
                ...getMobileStyles({
                  marginRight: "24px",
                }),
              },
            },
          }}
        >
          Compile
        </Button>
        {!isMobile && <MemoizedShareCodePanel disabled={isDownloading || noCode} />}
        {isMobile && (
          <div
            className={css({
              display: "flex",
              gap: "8px",
              paddingLeft: "24px",
              paddingRight: "24px",
            })}
          >
            <Button
              overrides={{
                Root: {
                  style: {
                    width: "50%",
                  },
                },
              }}
              kind={BUTTON_KIND.secondary}
              size={BUTTON_SIZE.large}
              onClick={() => setActiveComponent(LayoutComponent.Logs)}
            >
              Logs
            </Button>
            <Button
              overrides={{
                Root: {
                  style: {
                    width: "50%",
                  },
                },
              }}
              kind={BUTTON_KIND.secondary}
              size={BUTTON_SIZE.large}
              onClick={() => setActiveComponent(LayoutComponent.Contracts)}
            >
              Contracts
            </Button>
          </div>
        )}
      </div>
    </Card>
  );
};
