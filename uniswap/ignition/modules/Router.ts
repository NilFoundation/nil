import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

module.exports = buildModule("DeployUniswapV2Router01", (m) => {
  const router = m.contract("UniswapV2Router01");

  return { router };
});
