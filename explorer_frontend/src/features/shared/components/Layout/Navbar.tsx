import { useStore, useUnit } from "effector-react";
import { type ReactNode, memo } from "react";
import { useStyletron } from "styletron-react";
import { fetchSolidityCompiler } from "../../../../services/compiler";
import { CodeToolbar } from "../../../code/code-toolbar/CodeToolbar";
import { useCompileButton } from "../../../code/hooks/useCompileButton";
import { compile, compileCodeFx } from "../../../code/model";
import { tutorialWithUrlStringRoute } from "../../../routing/routes/tutorialRoute";
import { useMobile } from "../../hooks/useMobile";
import { Logo } from "./Logo";
import { styles } from "./styles";

type NavbarProps = {
  children?: ReactNode;
  showCodeInteractionButtons?: boolean;
};

const MemoizedCodeToolbar = memo(CodeToolbar);

export const Navbar = ({ children, showCodeInteractionButtons }: NavbarProps) => {
  const [css] = useStyletron();
  const [isDownloading, compiling] = useUnit([
    fetchSolidityCompiler.pending,
    compileCodeFx.pending,
  ]);
  const isTutorial = useStore(tutorialWithUrlStringRoute.$isOpened);
  const [isMobile] = useMobile();
  const templateColumns = isMobile ? "93% 1fr" : "1fr 33%";
  const btnTextContent = useCompileButton();
  return (
    <nav
      className={css({
        ...styles.navbar,
        gridTemplateColumns: templateColumns,
        gap: isMobile ? "0" : "8px",
        paddingLeft: isTutorial ? "26px" : "",
      })}
    >
      <div
        className={css({
          gridColumn: "1 / 2",
          display: "flex",
          flexGrow: 1,
          width: "100%",
          alignItems: "center",
        })}
      >
        <Logo />
        {showCodeInteractionButtons && (
          <MemoizedCodeToolbar
            disabled={isDownloading}
            isLoading={isDownloading || compiling}
            onCompileButtonClick={() => compile()}
            compileButtonContent={btnTextContent}
          />
        )}
      </div>
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
    </nav>
  );
};
