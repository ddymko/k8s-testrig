package commands

import (
	"github.com/spf13/cobra"
)

type properties struct {
	OrchestratorProfile *orchestratorProfile `json:"orchestratorProfile"`
	MasterProfile       *masterProfile       `json:"masterProfile"`
	AgentPoolProfiles   []agentPoolProfile   `json:"agentPoolProfiles"`
	LinuxProfile        *linuxProfile        `json:"linuxProfile"`
}

type orchestratorProfile struct {
	OrchestratorType    string            `json:"orchestratorType"`
	OrchestratorRelease string            `json:"orchestratorRelease"`
	KubernetesConfig    *kubernetesConfig `json:"kubernetesConfig"`
}

type kubernetesConfig struct {
	UseManagedIdentity bool   `json:"useManagedIdentity"`
	NetworkPlugin      string `json:"networkPlugin"`
	NetworkPolicy      string `json:"networkPolicy"`
	ContainerRuntime   string `json:"containerRuntime"`
}

type masterProfile struct {
	Count          int    `json:"count"`
	VMSize         string `json:"vmSize"`
	OSDiskSizeGB   int    `json:"osDiskSizeGB"`
	StorageProfile string `json:"storageProfile"`
	DNSPrefix      string `json:"dnsPrefix"`
}

type agentPoolProfile struct {
	Name                         string `json:"name"`
	Count                        int    `json:"count"`
	VMSize                       string `json:"vmSize"`
	OSDiskSizeGB                 int    `json:"osDiskSizeGB"`
	StorageProfile               string `json:"storageProfile"`
	AcceleratedNetworkingEnabled *bool  `json:"acceleratedNetworkingEnabled"`
	OSType                       string `json:"osType"`
	AvailabilityProfile          string `json:"availabilityProfile"`
}

type linuxProfile struct {
	AdminUsername string    `json:"adminUsername"`
	SSH           sshConfig `json:"ssh"`
}

type sshConfig struct {
	PublicKeys []sshKey `json:"publicKeys"`
}

type sshKey struct {
	KeyData string `json:"keyData"`
}

type apiModel struct {
	APIVersion string      `json:"apiVersion"`
	Properties *properties `json:"properties"`
}

// Defaults creates the `generate-defaults` subcommand.
// Use this to generate a default API model.
func Defaults() *cobra.Command {
	return &cobra.Command{
		Use: "generate-defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}

func defaultModel() *apiModel {
	return &apiModel{
		APIVersion: "vlabs",
		Properties: &properties{
			OrchestratorProfile: &orchestratorProfile{
				OrchestratorType:    "Kubernetes",
				OrchestratorRelease: "1.10",
				KubernetesConfig: &kubernetesConfig{
					UseManagedIdentity: true,
					NetworkPlugin:      "azure",
					NetworkPolicy:      "azure",
				},
			},
			MasterProfile: &masterProfile{
				Count:          3,
				VMSize:         "Standard_DS2_v2",
				StorageProfile: "ManagedDisks",
				OSDiskSizeGB:   200,
			},
			AgentPoolProfiles: []agentPoolProfile{
				agentPoolProfile{
					Name:                         "agentpool1",
					Count:                        3,
					VMSize:                       "Standard_DS2_v2",
					StorageProfile:               "ManagedDisks",
					OSDiskSizeGB:                 200,
					AvailabilityProfile:          "VirtualMachineScaleSets",
					AcceleratedNetworkingEnabled: boolPtr(true),
					OSType:                       "Linux",
				},
			},
			LinuxProfile: &linuxProfile{
				AdminUsername: "azureuser",
			},
		},
	}
}
