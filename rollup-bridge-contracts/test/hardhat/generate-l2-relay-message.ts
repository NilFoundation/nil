import { ethers } from "ethers";

// npx ts-node test/hardhat/generate-l2-relay-message.ts
export const generateL2RelayMessage = (depositorAddress: String, depositAmount: String, l2DepositRecipient: String, l2FeeRefundAddress: String): String => {
    const abi = [
        "function finaliseETHDeposit(address depositorAddress, uint256 depositAmount, address l2DepositRecipient, address l2FeeRefundAddress)"
    ];

    const iface = new ethers.Interface(abi);

    console.log(`about to generate depositMessage Data`);

    const depositMessage = iface.encodeFunctionData(
        "finaliseETHDeposit",
        [depositorAddress, depositAmount, l2DepositRecipient, l2FeeRefundAddress]
    );

    console.log(depositMessage);

    return depositMessage;
}


// const depositorAddressValue = "0xc8d5559BA22d11B0845215a781ff4bF3CCa0EF89";
// const depositAmountValue = "1000000000000";
// const l2DepositRecipientValue = "0x0001D3A5b915Bc99542a9430423cDe75Bd7F7aC7";
// const l2FeeRefundAddressValue = "0x000131b12EBeb7A34e1a47d402137df6De7b08Ae";

// generateL2RelayMessage(depositorAddressValue, depositAmountValue, l2DepositRecipientValue, l2FeeRefundAddressValue);