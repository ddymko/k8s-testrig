package commands

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/cpuguy83/strongerrors"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Remove creates a command to remove a cluster
// Note that this will remove the entire resource group!
func Remove(ctx context.Context, stateDir string, cfg *UserConfig) *cobra.Command {
	var (
		force          bool
		subscriptionID string
	)

	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a cluster",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if subscriptionID == "" {
				subscriptionID = cfg.Subscription
				if subscriptionID == "" {
					home, err := homedir.Dir()
					if err != nil {
						return errors.Wrap(err, "error determining home dir while trying to infer subscription ID")
					}
					subscriptionID, err = getSubFromAzDir(filepath.Join(home, ".azure"))
					if err != nil {
						return errors.Wrap(err, "no subscription provided and could not determine from azure CLI dir")
					}
				}
			}
			if err := runRemove(ctx, args, stateDir, subscriptionID, force, cmd.OutOrStdout()); err != nil {
				if !force {
					if !strongerrors.IsNotFound(err) {
						io.WriteString(cmd.OutOrStderr(), "Error while attempting remove.\nYou can verify the state details and try again, or use `--force` to remove all local state\n")
					}
				}
				return err
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&subscriptionID, "subscription", "s", "", "Set the subscription to use to deploy with")
	flags.BoolVarP(&force, "force", "f", false, "Force the removal of local state even if an error occurs when trying to remove from Azure")
	return cmd
}

func runRemove(ctx context.Context, names []string, stateDir, subscriptionID string, force bool, out io.Writer) error {
	auth, err := getAuthorizer()
	if err != nil {
		return err
	}

	gClient := resources.NewGroupsClient(subscriptionID)
	gClient.Authorizer = auth

	errors := make(chan error, len(names))
	var wg sync.WaitGroup
	wg.Add(len(names))

	for _, name := range names {
		go func(name string) {
			errors <- removeCluster(ctx, name, stateDir, gClient, force)
			wg.Done()
		}(name)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			io.WriteString(out, err.Error()+"\n")
		}
	}
	return nil
}

func removeCluster(ctx context.Context, name, stateDir string, client resources.GroupsClient, force bool) (retErr error) {
	dir := filepath.Join(stateDir, name)

	defer func() {
		if retErr == nil || force {
			removing := dir + ".removing"
			if err := os.Rename(dir, removing); err != nil && !os.IsNotExist(err) {
				if retErr == nil {
					retErr = err
				}
				return
			}
			if err := os.RemoveAll(removing); err != nil && !os.IsNotExist(err) {
				if retErr == nil {
					retErr = err
				}
				return
			}
			retErr = nil
		}
	}()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return clusterNotFound(name)
	}

	s, err := readState(dir)
	if err != nil {
		return err
	}
	if s.Status == stateInitialized || s.Status == stateCreating || s.Status == stateRemoving {
		return errors.Errorf("cannot remove while status is in state %q", strings.Title(string(s.Status)))
	}
	s.Status = stateRemoving
	defer func() {
		if retErr != nil {
			s.Status = stateDead
			writeState(dir, s)
		}
	}()
	writeState(dir, s)

	if s.ResourceGroup == "" {
		return errors.New("missing resource group in state object, cannot remove")
	}

	future, err := client.Delete(ctx, s.ResourceGroup)
	if err != nil {
		if isAzureNotFound(err) {
			err = nil
		}
		if err != nil {
			return errors.Wrapf(err, "error starting resource group deletion for %q", s.ResourceGroup)
		}
	} else {
		if err := future.WaitForCompletionRef(ctx, client.Client); err != nil {
			return errors.Wrapf(err, "error waiting for resource group deletion to finish for %q", s.ResourceGroup)
		}
	}

	return nil
}
