async function loadTutorials() {
  const [testTutorial, testContracts] = await Promise.all([
    import("./assets/tutorialOneText.md?raw"),
    import("./assets/tutorialOneContracts.sol?raw"),
  ]);
  const tutorials = [
    {
      stage: 1,
      text: testTutorial.default,
      contracts: testContracts.default,
    },
  ];
  return tutorials;
}

export default loadTutorials;
