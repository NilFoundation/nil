import { useStyletron } from "styletron-react";
import { Link } from "atomic-router-react";
import { styles } from "./styles";
import logo from "./assets/Logo.svg";
import { explorerRoute } from "../../../routing/routes/explorerRoute";

export const Logo = () => {
  const [css] = useStyletron();

  return (
    <Link className={css(styles.logo)} to={explorerRoute}>
      <img src={logo} alt="logo" />
    </Link>
  );
};
