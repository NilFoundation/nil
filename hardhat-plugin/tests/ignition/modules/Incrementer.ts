import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

module.exports = buildModule("IncrementerModule", (m: any) => {
  const incrementer = m.contract("Incrementer");

  return { incrementer };
});
