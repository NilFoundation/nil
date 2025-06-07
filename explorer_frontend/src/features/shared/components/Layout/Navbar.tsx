import { useUnit } from "effector-react";
import { type ReactNode, memo, useCallback } from "react";
import { useStyletron } from "styletron-react";
import { fetchSolidityCompiler } from "../../../../services/compiler";
import { getMobileStyles } from "../../../../styleHelpers";
import { CodeToolbar } from "../../../code/code-toolbar/CodeToolbar";
import { useCompileButton } from "../../../code/hooks/useCompileButton";
import { compile, compileCodeFx } from "../../../code/model";
import { useMobile } from "../../hooks/useMobile";
import { styles } from "./styles";

type NavbarProps = {
  children?: ReactNode;
  showCodeInteractionButtons?: boolean;
  logo?: ReactNode;
};

const MemoizedCodeToolbar = memo(CodeToolbar);

export const Navbar = ({ children, showCodeInteractionButtons, logo }: NavbarProps) => {
  const [css] = useStyletron();
  const [isDownloading, compiling] = useUnit([
    fetchSolidityCompiler.pending,
    compileCodeFx.pending,
  ]);
  const [isMobile] = useMobile();
  const templateColumns = isMobile ? "93% 1fr" : "1fr 33%";
  const btnTextContent = useCompileButton();

  const cb = useCallback(() => {
    compile();
  }, []);

  return (
    <nav
      className={css({
        ...styles.navbar,
        gridTemplateColumns: templateColumns,
        ...getMobileStyles({
          paddingTop: 0,
          gap: "8px",
        }),
      })}
    >
      <div
        className={css({
          gridColumn: "1 / 2",
          display: "flex",
          flexGrow: 1,
          width: "100%",
          alignItems: "center",
          ...getMobileStyles({
            gridColumn: "1 / -1",
            justifyContent: "space-between",
          }),
        })}
      >
        {logo}
        {showCodeInteractionButtons && (
          <MemoizedCodeToolbar
            disabled={isDownloading}
            isLoading={isDownloading || compiling}
            onCompileButtonClick={cb}
            compileButtonContent={btnTextContent}
          />
        )}
      </div>
      {children && (
        <div
          className={css({
            width: "auto",
            display: "flex",
            justifyContent: "end",
            alignItems: "center",
            marginLeft: isMobile ? "8px" : "0",
          })}
        >
          {children}
        </div>
      )}
    </nav>
  );
};
