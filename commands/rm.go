package commands

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Remove creates a command to remove a cluster
// Note that this will remove the entire resource group!
func Remove(ctx context.Context, stateDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(ctx, args[0], *stateDir)
		},
	}
	return cmd
}

func runRemove(ctx context.Context, name, stateDir string) error {
	return errors.New("not implemented")
}
