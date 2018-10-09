package commands

import (
	"context"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
)

// KubeConfig creates a a command to get the kubeconfig for a cluster
func KubeConfig(ctx context.Context, stateDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Get the path to the kubeconfig file for the specified cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKubeConfig(ctx, args[0], *stateDir, cmd.OutOrStdout())
		},
	}
	return cmd
}

func runKubeConfig(ctx context.Context, name string, stateDir string, outW io.Writer) error {
	dir := filepath.Join(stateDir, name)
	s, err := readState(dir)
	if err != nil {
		return err
	}
	io.WriteString(outW, filepath.Join(dir, "_output", "kubeconfig", "kubeconfig."+s.Location+".json"))
	return nil
}
