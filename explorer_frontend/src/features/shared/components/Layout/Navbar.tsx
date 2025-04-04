import type { ReactNode } from "react";
import { useStyletron } from "styletron-react";
import { Search } from "../../../search";
import { useMobile } from "../../hooks/useMobile";
import { styles } from "./styles";

type NavbarProps = {
  children?: ReactNode;
  logo?: ReactNode;
};

export const Navbar = ({ children, logo = null }: NavbarProps) => {
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
        {logo}
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
