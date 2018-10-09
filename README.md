# azure-k8s-testrig

`testrig` makes spinning up and managing test Kubernetes clusters on Azure simple.

### Usage:
```
testrig create --location=eastus myCluster
```

This will spin up a full kubernetes cluster on Azure.
This can be customized to some extent with the available CLI flags:

```
$ testrig create --help
Create a new kubernetes cluster on Azure

Usage:
  testrig create [flags]

      --acs-engine-path string                    Set the path to use for acs-engine (default "acs-engine")
  -h, --help                                      help for create
      --kubernetes-version string                 set the kubernetes version to use (default "1.10")
      --linux-agent-availability-profile string   set the availabiltiy profile for agent nodes (default "VirtualMachineScaleSets")
      --linux-agent-count int                     sets the number of nodes for agent pools (default 3)
      --linux-agent-node-sku string               sets sku to use for agent nodes (default "Standard_DS2_v2")
      --linux-leader-count int                    sets the number of nodes for the leader pool (default 3)
      --linux-leader-node-sku string              sets sku to use for agent nodes (default "Standard_DS2_v2")
  -l, --location string                           Set the location to deploy to
      --network-plugin string                     set the network plugin to use for the cluster (default "azure")
      --network-policy string                     set the network policy to use for the cluster (default "azure")
      --runtime string                            sets the containe runtime to use
```

#### Management commands

```
$ testrig --help
Quickly create and manage test Kubernetes clusters on Azure for testing purposes

Usage:
  testrig [command]

Available Commands:
  create      Create a new kubernetes cluster on Azure
  help        Help about any command
  inspect     Get details about an existing cluster
  kubeconfig  Get the path to the kubeconfig file for the specified cluster
  ls          List available clusters
  rm          Remove a cluster
  ssh         ssh into a running cluster

Flags:
  -h, --help               help for testrig
      --state-dir string   directory to store state information to
```

# Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

