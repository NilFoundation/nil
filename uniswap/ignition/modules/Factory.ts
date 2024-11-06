import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

module.exports = buildModule("DeployFactory", (m) => {
  const feeToSetter = m.getParameter(
    "feeToSetter",
    "0x0000000000000000000000000000000000000000",
  );
  const factory = m.contract("UniswapV2Factory", [feeToSetter]);

  return { factory };
});
