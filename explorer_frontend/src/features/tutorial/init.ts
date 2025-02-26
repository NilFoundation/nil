import { fetchTutorialFx } from "../code/model";
import { $tutorial } from "./model";

$tutorial.on(fetchTutorialFx.doneData, (_, tutorial) => tutorial);
