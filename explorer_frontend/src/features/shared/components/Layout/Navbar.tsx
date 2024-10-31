import { useStyletron } from "styletron-react";
import { styles } from "./styles";
import { Logo } from "./Logo";
import { Search } from "../../../search";
import { useMobile } from "../../hooks/useMobile";
import type { ReactNode } from "react";

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
