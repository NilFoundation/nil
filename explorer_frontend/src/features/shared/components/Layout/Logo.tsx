import { Link } from "atomic-router-react";
import type { ReactNode } from "react";
import { useStyletron } from "styletron-react";
import { explorerRoute } from "../../../routing/routes/explorerRoute";
import logo from "./assets/Logo.svg";
import { styles } from "./styles";

type LogoProps = {
  subText?: ReactNode;
};

export const Logo = ({ subText }: LogoProps) => {
  const [css] = useStyletron();

  return (
    <Link className={css(styles.logo)} to={explorerRoute}>
      <img src={logo} alt="logo" />
      {subText}
    </Link>
  );
};
