import type { ReactNode } from "react";
import { useStyletron } from "styletron-react";
import { Search } from "../../../search";
import { useMobile } from "../../hooks/useMobile";
import { Logo } from "./Logo";
import { styles } from "./styles";

type NavbarProps = {
  children?: ReactNode;
};

export const Navbar = ({ children }: NavbarProps) => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();
  return (
    <nav className={css(styles.navbar)}>
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
        {!isMobile && <Search />}
      </div>
      <div
        className={css({
          gridColumn: "2 / 3",
        })}
      >
        {children}
      </div>
    </nav>
  );
};
