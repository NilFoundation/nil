import { useStyletron } from "styletron-react";
import { mobileContainerStyle, mobileContentStyle, styles } from "./styles";
import { Navbar } from "./Navbar";
import { useMobile } from "../../hooks/useMobile";
import type { ReactNode } from "react";

type LayoutProps = {
  children: ReactNode;
  sidebar?: ReactNode;
  navbar?: ReactNode;
};

export const Layout = ({ children, sidebar, navbar }: LayoutProps) => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  return (
    <div className={css(isMobile ? mobileContainerStyle : styles.container)}>
      <Navbar>{navbar}</Navbar>
      <div className={css(isMobile ? mobileContentStyle : styles.content)}>
        {sidebar}
        {children}
      </div>
    </div>
  );
};
