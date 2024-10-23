export const formatSolidityError = (error: string) => {
  const lines = error.split("\n");

  const formattedLines: string[] = [];

  lines.forEach((line, index) => {
    if (index === 0) {
      formattedLines.push(line);
    } else if (line.includes("-->")) {
      formattedLines.push(`Location: ${line.trim()}`);
    }
  });

  return formattedLines.join("\n");
};
