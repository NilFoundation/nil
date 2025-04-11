# Async deploy between shards

In =nil;, it is possible to deploy smart contracts from other smart contracts. This operation can extend across shards.

To initiate such an async deployment, use the *Nil.asyncDeploy()* function:

```solidity
function asyncDeploy(
    uint shardId,
    address bounceTo,
    uint value,
    bytes memory code,
    uint256 salt
) internal returns (address)
```

## Task

* *Counter*
* *Deployer*

*Counter* is a simple 'incrementer' contract. It should not be modified.

*Deployer* only has one function (*deploy()*) which is a 'wrapper' function over *Nil.asyncDeploy()*. 

To complete this tutorial:

* Finish the *Deployer* contract by completing the *deploy()* function. The function should accept bytecode and deploy a contract with said bytecode.

## Checks

This tutorial is verified once the following checks are passed.

* *Deployer* and *Counter* are compiled.
* *Deployer* is deployed.
* *Deployer* successfully deploys *Counter* contract via the *deploy()* function.
* The deployed *Counter* can increment its value.

To run these checks:

1. Compile both contacts
2. Click on 'Run Checks'