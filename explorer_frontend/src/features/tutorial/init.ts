import { combine, sample } from "effector";
import { persist } from "effector-storage/local";
import { changeCode, loadedTutorialPage } from "../code/model";
import { $contracts } from "../contracts/models/base";
import { notFoundRoute } from "../routing/routes/routes";
import { tutorialWithUrlStringRoute } from "../routing/routes/tutorialRoute";
import {
  $completedTutorials,
  $tutorial,
  $tutorialUserSolutions,
  $tutorials,
  fetchAllTutorialsFx,
  fetchTutorialEvent,
  fetchTutorialFx,
  setCompletedTutorial,
} from "./model";

$tutorial.on(fetchTutorialFx.doneData, (_, tutorial) => tutorial);
$tutorials.on(fetchAllTutorialsFx.doneData, (_, tutorials) => tutorials);

persist({
  key: "completedTutorials",
  store: $completedTutorials,
});

persist({
  key: "userSolutions",
  store: $tutorialUserSolutions,
});

sample({
  clock: [loadedTutorialPage, tutorialWithUrlStringRoute.$params],
  source: tutorialWithUrlStringRoute.$params,
  fn: (params) => params.urlSlug,
  filter: (urlSlug) => urlSlug !== undefined,
  target: fetchTutorialFx,
});

sample({
  clock: loadedTutorialPage,
  target: fetchAllTutorialsFx,
});

sample({
  clock: fetchTutorialEvent,
  source: fetchTutorialEvent,
  fn: (tutorial) => tutorial.urlSlug.toString(),
  target: fetchTutorialFx,
});

sample({
  clock: fetchTutorialFx.doneData,
  source: combine($tutorial, $tutorialUserSolutions),
  filter: ([tutorial, userSolutions]) => {
    const res = Object.keys(userSolutions).includes(tutorial.urlSlug);
    return !res;
  },
  fn: ([tutorial]) => tutorial.contracts,
  target: changeCode,
});

sample({
  clock: fetchTutorialFx.doneData,
  source: combine($tutorial, $tutorialUserSolutions),
  filter: ([tutorial, userSolutions]) => {
    const res = Object.keys(userSolutions).includes(tutorial.urlSlug);
    return res;
  },
  fn: ([tutorial, userSolutions]) => {
    const solutions = userSolutions[tutorial.urlSlug];
    return solutions;
  },
  target: changeCode,
});

sample({
  clock: fetchTutorialFx.failData,
  fn: () => notFoundRoute.open(),
});

sample({
  clock: setCompletedTutorial,
  fn: (stage) => {
    const completedTutorials = $completedTutorials.getState();
    if (!completedTutorials.includes(stage)) {
      return [...completedTutorials, stage];
    }
    return completedTutorials;
  },
  target: $completedTutorials,
});

sample({
  clock: setCompletedTutorial,
  source: combine($contracts, tutorialWithUrlStringRoute.$params, $tutorialUserSolutions),
  fn: ([userSolutions, urlSlug, currentSolutions]) => {
    return {
      ...currentSolutions,
      [urlSlug.urlSlug]: userSolutions.at(0)?.sourcecode || "",
    };
  },
  target: $tutorialUserSolutions,
});
