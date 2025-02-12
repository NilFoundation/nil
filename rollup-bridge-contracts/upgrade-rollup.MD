# NilRollup Upgrade Procedure

- NilRollup is a upgradeableProxy contract and deployed using `TransparentUpgradeableProxy` pattern 
- Proxy is managed by another contract `ProxyAdmin`  
- `ProxyAdmin` is a contract which gets deployed exclusively and on the fly when the proxy of the `NilRollup` contract gets deployed  
- `ProxyAdmin` is controlled and owned by the deployer account i.e the account which has sent the deploy transaction for proxy  
- This is documentation with instructions on how to upgrade the existing NilRollup Proxy contract


## Pre-Requisites

1. Registration

  - The registration step using `upgrades.forceImport`is necessary when the OpenZeppelin Upgrades plugin does not have the proxy's 
      deployment information stored in its local manifest.  
  - This can happen if the proxy was deployed using a different tool or if the local manifest was lost or corrupted.    
  - The `forceImport` function registers the existing proxy with the OpenZeppelin Upgrades plugin, allowing it to manage and upgrade
    the proxy.  

  **Purpose**: `forceImport` is used to register an existing proxy with the OpenZeppelin Upgrades plugin.  
  **When to Use**: Use `forceImport` when the proxy was deployed using a different tool or if the local manifest is missing the proxy's deployment information.  
  **How It Works**: `forceImport`takes the proxy address and the contract factory, and it registers the proxy with the plugin, allowing it to manage and upgrade the proxy.  

2. Upgrader

   - The transaction maker should be the same address who has deployed the proxy contract
   - As the ProxyAdmin contract's owner gets defaulted to the deployer address who has initiated the ProxyDeployment
   - ensure that the `.env` has the privateKey is same as the one used during deployment of proxy contract


