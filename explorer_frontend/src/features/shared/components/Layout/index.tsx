import type { ReactNode } from "react";
import { useStyletron } from "styletron-react";
import { useMobile } from "../../hooks/useMobile";
import { Logo } from "./Logo";
import { Navbar } from "./Navbar";
import { mobileContainerStyle, mobileContentStyle, styles } from "./styles";

type LayoutProps = {
  children: ReactNode;
  navbar?: ReactNode;
};

export const Layout = ({ children, navbar }: LayoutProps) => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  return (
    <div className={css(isMobile ? mobileContainerStyle : styles.container)}>
      <Navbar logo={<Logo />}>{navbar}</Navbar>
      <div className={css(isMobile ? mobileContentStyle : styles.content)}>{children}</div>
    </div>
  );
};
