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

			return runCreate(ctx, args[0], opts, os.Stdin, cmd.OutOrStdout(), cmd.OutOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.ACSEnginePath, "acs-engine-path", "acs-engine", "Set the path to use for acs-engine")

	// TODO(@cpuguy83): Configure this through some default config in the state dir
	// flags.StringVarP(&opts.ResourceGroup, "resource-group", "g", "testrig", "Set the resource group to deploy to. If the group doesn't exist, it will be created")
	flags.StringVarP(&opts.Location, "location", "l", "", "Set the location to deploy to")

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

	return cmd
}

type createOpts struct {
	StateDir      string
	Model         *apiModel
	ACSEnginePath string
	Location      string
}

func runCreate(ctx context.Context, name string, opts createOpts, in io.Reader, outW, errW io.Writer) error {
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

	s := state{
		Status:        stateInitialized,
		Location:      opts.Location,
		ResourceGroup: dnsName,
	}

	dir := filepath.Join(opts.StateDir, name)
	if _, err := os.Stat(dir); err == nil {
		return errors.Errorf("cluster with name %q already exists", name)
	}

	if err := os.Mkdir(dir, 0700); err != nil {
		return errors.Wrapf(err, "error creating state dir %s", dir)
	}

	statePath := filepath.Join(dir, "state.json")
	if err := writeState(statePath, s); err != nil {
		return err
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
	if err := writeState(statePath, s); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, opts.ACSEnginePath, "deploy",
		"--api-model", modelPath,
		"--location", opts.Location,
		"--resource-group", s.ResourceGroup,
		"--output-directory", filepath.Join(dir, "_output"),
		"--dns-prefix", dnsName,
	)

	buf := bytes.NewBuffer(nil)

	// wire up i/o here because `acs-engine` might ask to login
	// TODO(@cpuguy83): Support login from testrig instead of going through acs-engine
	cmd.Stdin = in
	cmd.Stdout = io.MultiWriter(outW, buf)
	cmd.Stderr = io.MultiWriter(errW, buf)

	if err := cmd.Run(); err != nil {
		s.Status = stateFailure
		s.FailureMessage = buf.String()
		writeState(statePath, s)
		return errors.Wrapf(err, "%s exited with error", filepath.Base(opts.ACSEnginePath))
	}

	s.Status = stateReady
	s.DNSPrefix = dnsName
	if err := writeState(statePath, s); err != nil {
		return errors.Wrap(err, "create succeeded but received error while writing state")
	}

	return nil
}
