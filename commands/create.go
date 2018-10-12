package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Create creates the `create` subcommand.
func Create(ctx context.Context, stateDir string, cfg *UserConfig) *cobra.Command {
	m := defaultModel()
	configErr := overrideModelDefaults(m, cfg)
	var opts createOpts

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new kubernetes cluster on Azure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if configErr != nil {
				return configErr
			}
			if _, err := os.Stat(opts.ACSEnginePath); err != nil && os.IsNotExist(err) {
				var err2 error
				if opts.ACSEnginePath, err2 = exec.LookPath(opts.ACSEnginePath); err2 != nil {
					return errors.New("could not find acs-engine binary")
				}
			}

			if opts.SubscriptionID == "" {
				opts.SubscriptionID = cfg.Subscription
				if opts.SubscriptionID == "" {
					home, err := homedir.Dir()
					if err != nil {
						return errors.Wrap(err, "error determining home dir while trying to infer subscription ID")
					}
					opts.SubscriptionID, err = getSubFromAzDir(filepath.Join(home, ".azure"))
					if err != nil {
						return errors.Wrap(err, "no subscription provided and could not determine from azure CLI dir")
					}
				}
			}

			if opts.Location == "" {
				opts.Location = cfg.Location
			}
			opts.StateDir = stateDir
			opts.Model = m

			return runCreate(ctx, args[0], opts, os.Stdin, cmd.OutOrStdout(), cmd.OutOrStderr())
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.ACSEnginePath, "acs-engine-path", "acs-engine", "Location of acs-engine binary")

	// TODO(@cpuguy83): Configure this through some default config in the state dir
	flags.StringVarP(&opts.Location, "location", "l", cfg.Location, "Azure location to deploy to, e.g. `centralus` (required)")
	flags.StringVarP(&opts.SubscriptionID, "subscription", "s", "", "Azure subscription to deploy the cluster with")

	p := m.Properties
	flags.IntVar(&p.MasterProfile.Count, "linux-leader-count", p.MasterProfile.Count, "Number of nodes for the Kubernetes leader pool")
	flags.StringVar(&p.MasterProfile.VMSize, "linux-leader-node-sku", p.MasterProfile.VMSize, "VM SKU for leader nodes")

	flags.IntVar(&p.AgentPoolProfiles[0].Count, "linux-agent-count", p.AgentPoolProfiles[0].Count, "Number of Linux nodes for the Kubernetes agent/worker pools")
	flags.StringVar(&p.AgentPoolProfiles[0].VMSize, "linux-agent-node-sku", p.AgentPoolProfiles[0].VMSize, "VM SKU for Linux agent nodes")
	flags.StringVar(&p.AgentPoolProfiles[0].AvailabilityProfile, "linux-agent-availability-profile", p.AgentPoolProfiles[0].AvailabilityProfile, "Availabiltiy profile for Linux agent nodes")

	flags.IntVar(&p.AgentPoolProfiles[1].Count, "windows-agent-count", p.AgentPoolProfiles[1].Count, "Number of Windows nodes for the Kubernetes agent/worker pools")
	flags.StringVar(&p.AgentPoolProfiles[1].VMSize, "windows-agent-node-sku", p.AgentPoolProfiles[1].VMSize, "VM SKU for Windows agent nodes")
	flags.StringVar(&p.AgentPoolProfiles[1].AvailabilityProfile, "windows-agent-availability-profile", p.AgentPoolProfiles[1].AvailabilityProfile, "Availabiltiy profile for Windows agent nodes")

	flags.StringVar(&p.OrchestratorProfile.KubernetesConfig.ContainerRuntime, "runtime", p.OrchestratorProfile.KubernetesConfig.ContainerRuntime, "Container runtime to use")
	flags.StringVar(&p.OrchestratorProfile.KubernetesConfig.NetworkPlugin, "network-plugin", p.OrchestratorProfile.KubernetesConfig.NetworkPlugin, "Network plugin to use for the cluster")
	flags.StringVar(&p.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "network-policy", p.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "Network policy to use for the cluster")

	flags.StringVar(&p.OrchestratorProfile.OrchestratorRelease, "kubernetes-version", p.OrchestratorProfile.OrchestratorRelease, "Specify the Kubernetes version")

	flags.StringVarP(&p.LinuxProfile.AdminUsername, "user", "u", p.LinuxProfile.AdminUsername, "Username for SSH access to nodes")
	flags.Var(&p.LinuxProfile.SSH, "ssh-key", "Public SSH key to install as an authorized key on cluster nodes")

	return cmd
}

type createOpts struct {
	StateDir       string
	Model          *apiModel
	ACSEnginePath  string
	Location       string
	SubscriptionID string
	ResourceGroup  string
}

func runCreate(ctx context.Context, name string, opts createOpts, in io.Reader, outW, errW io.Writer) (retErr error) {
	var deletePools []int
	for i, p := range opts.Model.Properties.AgentPoolProfiles {
		if p.Count == 0 {
			deletePools = append(deletePools, i)
		}
	}

	for n, i := range deletePools {
		opts.Model.Properties.AgentPoolProfiles = append(opts.Model.Properties.AgentPoolProfiles[:i-n], opts.Model.Properties.AgentPoolProfiles[i-n+1:]...)
	}

	if len(opts.Model.Properties.AgentPoolProfiles) == 0 {
		return errors.New("must have at least 1 agent node")
	}

	var s state
	dir := filepath.Join(opts.StateDir, name)

	defer func() {
		if retErr == nil {
			return
		}

		s.Status = stateFailure
		if s.FailureMessage == "" {
			s.FailureMessage = retErr.Error()
		}
		writeState(dir, s)
	}()

	if opts.Location == "" {
		return errors.New("Must specify a location")
	}

	if err := os.MkdirAll(opts.StateDir, 0755); err != nil {
		return errors.Wrap(err, "error creating state dir")
	}

	random, err := generateRandom()
	if err != nil {
		return err
	}

	dnsName := name + "-" + random
	opts.Model.Properties.MasterProfile.DNSPrefix = dnsName

	if opts.ResourceGroup == "" {
		opts.ResourceGroup = dnsName
	}

	s = state{
		Status:        stateInitialized,
		Location:      opts.Location,
		ResourceGroup: opts.ResourceGroup,
		CreatedAt:     time.Now(),
	}

	if _, err := os.Stat(dir); err == nil {
		return errors.Errorf("cluster with name %q already exists", name)
	}

	if err := os.Mkdir(dir, 0700); err != nil {
		return errors.Wrapf(err, "error creating state dir %s", dir)
	}

	if err := writeState(dir, s); err != nil {
		return err
	}

	if len(opts.Model.Properties.LinuxProfile.SSH.PublicKeys) == 0 {
		keyPath := filepath.Join(dir, "id_rsa")
		f, err := os.OpenFile(keyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return errors.Wrap(err, "could not create private SSH key file and no ssh keys were provided")
		}
		defer f.Close()
		pubKey, err := createSSHKey(f)
		if err != nil {
			return errors.Wrap(err, "error creating SSH key and no SSH key was provided")
		}
		s.SSHIdentityFile = keyPath
		if err := writeState(dir, s); err != nil {
			return err
		}
		opts.Model.Properties.LinuxProfile.SSH.PublicKeys = append(opts.Model.Properties.LinuxProfile.SSH.PublicKeys, sshKey{KeyData: pubKey})
	}

	for _, p := range opts.Model.Properties.AgentPoolProfiles {
		if p.Count > 0 {
			switch strings.ToLower(p.OSType) {
			case "linux":
				// ssh key is already generated since leader nodes are linux
			case "windows":
				if opts.Model.Properties.WindowsProfile.AdminPassword == "" {
					opts.Model.Properties.WindowsProfile.AdminPassword, err = generatePassword()
					if err != nil {
						return errors.Wrap(err, "error generating random password for Windows admin user")
					}
				}
			}
		}
	}

	modelJSON, err := json.MarshalIndent(opts.Model, "", "\t")
	if err != nil {
		return errors.Wrap(err, "error marshalling api model")
	}
	modelPath := filepath.Join(dir, "apimodel.json")
	// This file may contain a password in it, so make sure it's not readable by anyone but the user.
	if err := ioutil.WriteFile(modelPath, modelJSON, 0600); err != nil {
		return errors.Wrap(err, "error writing API model to disk")
	}

	s.Status = stateCreating
	if err := writeState(dir, s); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, opts.ACSEnginePath, "generate",
		"--output-directory", filepath.Join(dir, "_output"),
		"--api-model", modelPath,
	)

	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	cmd.Stderr = buf

	if err := cmd.Run(); err != nil {
		s.Status = stateFailure
		s.FailureMessage = buf.String()
		writeState(dir, s)
		return errors.Wrapf(err, "%s exited with error: %s", filepath.Base(opts.ACSEnginePath), s.FailureMessage)
	}

	auth, err := getAuthorizer()
	if err != nil {
		return err
	}

	gClient := resources.NewGroupsClient(opts.SubscriptionID)
	if err != nil {
		return errors.Wrap(err, "error creating resources client")
	}

	gClient.Authorizer = auth
	if _, err := gClient.CreateOrUpdate(ctx, opts.ResourceGroup, resources.Group{Location: &opts.Location, Name: &dnsName}); err != nil {
		return errors.Wrapf(err, "error creating resource group %q", dnsName)
	}

	template, params, err := readACSDeployment(dir)
	if err != nil {
		return err
	}

	dClient := resources.NewDeploymentsClient(opts.SubscriptionID)
	dClient.Authorizer = auth
	future, err := dClient.CreateOrUpdate(ctx, opts.ResourceGroup, dnsName, resources.Deployment{
		Properties: &resources.DeploymentProperties{Template: &template, Parameters: &params, Mode: resources.Incremental},
	})
	if err != nil {
		return errors.Wrap(err, "error creating deployment")
	}

	if err := future.WaitForCompletionRef(ctx, dClient.Client); err != nil {
		return errors.Wrap(err, "error in deployment")
	}
	deployment, err := future.Result(dClient)
	if err != nil {
		return errors.Wrap(err, "error getting deployment result")
	}

	s.DeploymentName = *deployment.Name
	s.Status = stateReady
	s.DNSPrefix = dnsName
	if err := writeState(dir, s); err != nil {
		return errors.Wrap(err, "create succeeded but received error while writing state")
	}

	return nil
}
