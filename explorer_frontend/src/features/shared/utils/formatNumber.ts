const formatNumber = (number: number) => {
  return new Intl.NumberFormat("en-US").format(number);
};

function formatToK(input: number): string {
  if (Number.isNaN(input)) throw new Error("Input is not a number");

  if (input >= 1_000_000) return `${(input / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
  if (input >= 1_000) return `${(input / 1_000).toFixed(1).replace(/\.0$/, "")}K`;

  return formatNumber(input);
}

export { formatNumber, formatToK };
