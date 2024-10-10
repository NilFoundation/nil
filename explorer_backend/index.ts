import "./services/opentelemtry";
export type { AppRouter } from "./routes";

const init = () => {
  import("./start");
};

init();
