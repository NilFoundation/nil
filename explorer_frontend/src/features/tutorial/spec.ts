import { TutorialLevel } from "./model";
import {
  tutorialFourContracts,
  tutorialFourIcon,
  tutorialFourText,
  tutorialOneContracts,
  tutorialOneIcon,
  tutorialOneText,
  tutorialThreeContracts,
  tutorialThreeIcon,
  tutorialThreeText,
  tutorialTwoContracts,
  tutorialTwoIcon,
  tutorialTwoText,
} from "./tutorialImports";

async function loadTutorials() {
  const tutorials = [
    {
      stage: 1,
      text: tutorialOneText,
      contracts: tutorialOneContracts,
      icon: tutorialOneIcon,
      completionTime: "5 minutes",
      level: TutorialLevel.Easy,
      title: "Async calls and default tokens",
      description: "Send an async call between shards.",
      urlSlug: "async-call",
    },
    {
      stage: 2,
      text: tutorialTwoText,
      contracts: tutorialTwoContracts,
      icon: tutorialTwoIcon,
      completionTime: "8 minutes",
      level: TutorialLevel.Easy,
      title: "Working with custom tokens",
      description: "Mint and send custom tokens across shards.",
      urlSlug: "custom-tokens",
    },
    {
      stage: 3,
      text: tutorialThreeText,
      contracts: tutorialThreeContracts,
      icon: tutorialThreeIcon,
      completionTime: "8 minutes",
      level: TutorialLevel.Medium,
      title: "Async deploy",
      description: "Deploy contracts asynchronously.",
      urlSlug: "async-deploy",
    },
    {
      stage: 4,
      text: tutorialFourText,
      contracts: tutorialFourContracts,
      icon: tutorialFourIcon,
      completionTime: "8 minutes",
      level: TutorialLevel.Medium,
      title: "NFTs and cross-shard transfers",
      description: "Send an NFT across shards.",
      urlSlug: "send-nft",
    },
  ];

  return tutorials;
}

export default loadTutorials;
