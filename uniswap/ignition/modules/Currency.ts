import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

module.exports = buildModule("DeployCurrency", (m) => {
  const token = m.contract("Currency", [
    m.getParameter("currencyName", ""),
    m.getParameter("pubkey", ""),
  ]);

  return { token };
});
