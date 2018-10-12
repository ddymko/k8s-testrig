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

Flags:
      --acs-engine-path string                      Location of acs-engine binary (default "acs-engine")
  -h, --help                                        help for create
      --kubernetes-version string                   Specify the Kubernetes version (default "1.10")
      --linux-agent-availability-profile string     Availabiltiy profile for Linux agent nodes (default "VirtualMachineScaleSets")
      --linux-agent-count int                       Number of Linux nodes for the Kubernetes agent/worker pools (default 3)
      --linux-agent-node-sku string                 VM SKU for Linux agent nodes (default "Standard_DS2_v2")
      --linux-leader-count int                      Number of nodes for the Kubernetes leader pool (default 3)
      --linux-leader-node-sku string                VM SKU for leader nodes (default "Standard_DS2_v2")
  -l, --location centralus                          Azure location to deploy to, e.g. centralus (required)
      --network-plugin string                       Network plugin to use for the cluster (default "azure")
      --network-policy string                       Network policy to use for the cluster (default "azure")
      --runtime string                              Container runtime to use
      --ssh-key sshKey                              Public SSH key to install as an authorized key on cluster nodes
  -s, --subscription string                         Azure subscription to deploy the cluster with
  -u, --user string                                 Username for SSH access to nodes (default "azureuser")
      --windows-agent-availability-profile string   Availabiltiy profile for Windows agent nodes (default "VirtualMachineScaleSets")
      --windows-agent-count int                     Number of Windows nodes for the Kubernetes agent/worker pools
      --windows-agent-node-sku string               VM SKU for Windows agent nodes (default "Standard_DS2_v3")

Global Flags:
      --config string      Location of user config file (default "/Users/cpuguy83/.testrig/config.toml")
      --state-dir string   Directory to store state information to (default "/Users/cpuguy83/.testrig")
```

When creating a cluster you can provide your own (public) ssh key or a key pair will be generated for you.

#### Authentication

`testrig` attempts to setup authentcation in the following order:
1. Read service principal from the `AZURE_AUTH_LOCATION`
2. Receive a bearer token through azure-cli

If method `1` fails (e.g. if the value is unset), then method 2 is attempted.

To make a service principal suitable for `1`, run:

```
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}"
```

The output of this should be saved to a file and the file path passed into `testrig` as an environment variable as `AZURE_AUTH_LOCATION`.

For method `2`, you must make sure the azure-cli is logged in: `az login`.

Commands will use whatever subscription you are logged in with, or you can pass in a custom subscription.

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

#### User supplied defaults

In addition to overriding defaults via flags, users can also supply a default config file.
By default, `testrig` will look in `~/.testrig/config.toml` (or equiv home dir on Windows), but can also be supplied as a flag: `--config`

The config should be in toml format, with the following struct definition:

```go
// UserConfig represents the user configuration read from a config file
type UserConfig struct {
	Subscription string
	Location     string

	Profile struct {
		KubernetesVersion string
		Leader            struct {
			Linux struct {
				SKU   string
				Count *int
			}
		}
		Agent struct {
			Linux   AgentNodeConfig
			Windows AgentNodeConfig
		}
		Auth struct {
			Linux struct {
				User          string
				PublicKeyFile string
			}
			Windows struct {
				User         string
				PasswordFile string
			}
		}
	}
}

// AgentNodeConfig is used to configure an agent node pool.
// It's used by UserConfig
type AgentNodeConfig struct {
	SKU   string
	Count *int
}
```

Example Config:

```toml
Location = "centralus"

[profile]
  KubernetesVersion = "1.11"
			[profile.leader.linux]
				count = 1
```

Tabs or spaces, capitalization, doesn't matter.

### Install

This project uses go modules, introduced in go1.11. While you can build prior versions of go, this is not tested against and will require fetching depdendencies.

Using `go get`:

```
go get github.com/Azure/k8s-testrig
```

Build Locally:

```
make build
make install
```

Using docker:

```
make docker-build
make install
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
