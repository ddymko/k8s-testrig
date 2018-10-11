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
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Create creates the `create` subcommand.
func Create(ctx context.Context, stateDir *string) *cobra.Command {
	m := defaultModel()
	var opts createOpts

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new kubernetes cluster on Azure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(opts.ACSEnginePath); err != nil && os.IsNotExist(err) {
				var err2 error
				if opts.ACSEnginePath, err2 = exec.LookPath(opts.ACSEnginePath); err2 != nil {
					return errors.New("could not find acs-engine binary")
				}
			}
			opts.Model = m
			opts.StateDir = *stateDir

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

			return runCreate(ctx, args[0], opts, os.Stdin, cmd.OutOrStdout(), cmd.OutOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.ACSEnginePath, "acs-engine-path", "acs-engine", "Set the path to use for acs-engine")

	// TODO(@cpuguy83): Configure this through some default config in the state dir
	// flags.StringVarP(&opts.ResourceGroup, "resource-group", "g", "testrig", "Set the resource group to deploy to. If the group doesn't exist, it will be created")
	flags.StringVarP(&opts.Location, "location", "l", "", "Set the location to deploy to")
	flags.StringVarP(&opts.SubscriptionID, "subscription", "s", "", "Set the subscription to use to deploy with")

	p := m.Properties
	flags.IntVar(&p.MasterProfile.Count, "linux-leader-count", p.MasterProfile.Count, "sets the number of nodes for the leader pool")
	flags.StringVar(&p.MasterProfile.VMSize, "linux-leader-node-sku", p.MasterProfile.VMSize, "sets sku to use for agent nodes")

	flags.IntVar(&p.AgentPoolProfiles[0].Count, "linux-agent-count", p.AgentPoolProfiles[0].Count, "sets the number of nodes for agent pools")
	flags.StringVar(&p.AgentPoolProfiles[0].VMSize, "linux-agent-node-sku", p.AgentPoolProfiles[0].VMSize, "sets sku to use for agent nodes")
	flags.StringVar(&p.AgentPoolProfiles[0].AvailabilityProfile, "linux-agent-availability-profile", p.AgentPoolProfiles[0].AvailabilityProfile, "set the availabiltiy profile for agent nodes")

	flags.StringVar(&p.OrchestratorProfile.KubernetesConfig.ContainerRuntime, "runtime", p.OrchestratorProfile.KubernetesConfig.ContainerRuntime, "sets the containe runtime to use")
	flags.StringVar(&p.OrchestratorProfile.KubernetesConfig.NetworkPlugin, "network-plugin", p.OrchestratorProfile.KubernetesConfig.NetworkPlugin, "set the network plugin to use for the cluster")
	flags.StringVar(&p.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "network-policy", p.OrchestratorProfile.KubernetesConfig.NetworkPolicy, "set the network policy to use for the cluster")
	flags.StringVar(&p.OrchestratorProfile.OrchestratorRelease, "kubernetes-version", p.OrchestratorProfile.OrchestratorRelease, "set the kubernetes version to use")
	flags.StringVarP(&p.LinuxProfile.AdminUsername, "user", "u", p.LinuxProfile.AdminUsername, "set the username to use for nodes")
	flags.Var(&p.LinuxProfile.SSH, "ssh-key", "set public SSH key to install as authorized keys in cluster nodes")

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
		pubKey, err := createSSHKey(ctx, f)
		if err != nil {
			return errors.Wrap(err, "error creating SSH key and no SSH key was provided")
		}
		s.SSHIdentityFile = keyPath
		if err := writeState(dir, s); err != nil {
			return err
		}
		opts.Model.Properties.LinuxProfile.SSH.PublicKeys = append(opts.Model.Properties.LinuxProfile.SSH.PublicKeys, sshKey{KeyData: pubKey})
	}

	modelJSON, err := json.MarshalIndent(opts.Model, "", "\t")
	if err != nil {
		return errors.Wrap(err, "error marshalling api model")
	}
	modelPath := filepath.Join(dir, "apimodel.json")
	if err := ioutil.WriteFile(modelPath, modelJSON, 0644); err != nil {
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
		return errors.Wrapf(err, "%s exited with error", filepath.Base(opts.ACSEnginePath))
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
