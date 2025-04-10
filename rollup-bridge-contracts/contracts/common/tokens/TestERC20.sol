// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/// @title TestERC20Token
/// @notice ERC20Token used for periphery tests
contract TestERC20Token is ERC20 {
  // Internal variable to store decimals configuration
  uint8 _decimals;

  /// @dev Constructor initializes the token with a name, symbol, and decimals.
  /// @param name_ The name of the token.
  /// @param symbol_ The symbol of the token.
  /// @param decimals_ The number of decimals used for token calculations.
  constructor(string memory name_, string memory symbol_, uint8 decimals_) ERC20(name_, symbol_) {
    _decimals = decimals_;
  }

  /// @dev Returns the number of decimals used by the token.
  /// @return The number of decimals.
  function decimals() public view virtual override returns (uint8) {
    return _decimals;
  }

  /// @dev Mints new tokens and assigns them to the specified account.
  /// @param account The account to receive the minted tokens.
  /// @param amount The amount of tokens to mint.
  function mint(address account, uint256 amount) public {
    _mint(account, amount);
  }

  /// @dev Sets the number of decimals used by the token.
  /// Only the contract owner can call this function.
  /// @param decimals_ The new number of decimals.
  function setDecimals(uint8 decimals_) public {
    _decimals = decimals_;
  }

  /// @dev Deposits Ether into the contract and mints corresponding tokens to the sender.
  function deposit() external payable {
    depositTo(msg.sender);
  }

  /// @dev Withdraws an amount of tokens from the contract and sends Ether back to the sender.
  /// @param amount The amount of tokens to withdraw.
  function withdraw(uint256 amount) external {
    withdrawTo(msg.sender, amount);
  }

  /// @dev Deposits Ether into the contract and mints corresponding tokens to the specified account.
  /// @param account The account to receive the minted tokens and Ether deposit.
  function depositTo(address account) public payable {
    _mint(account, msg.value);
  }

  /// @dev Withdraws an amount of tokens from the contract and sends Ether to the specified account.
  /// @param account The account to receive the withdrawn Ether.
  /// @param amount The amount of tokens to withdraw.
  function withdrawTo(address account, uint256 amount) public {
    _burn(msg.sender, amount);
    (bool success, ) = account.call{ value: amount }("");
    require(success, "FAIL_TRANSFER");
  }
}
