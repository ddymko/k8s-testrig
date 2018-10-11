package commands

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cpuguy83/strongerrors"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// SSH creates the command to ssh into the cluster
func SSH(ctx context.Context, stateDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ssh",
		Example: "ssh <name> -- <ssh args>",
		Short:   "ssh into a running cluster",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			var sshArgs []string
			if len(args) > 1 {
				sshArgs = args[1:]
				for _, arg := range args {
					if arg == "-i" {
						return errors.New("must not provide `-i` flag to ssh args")
					}
				}
			}
			return runSSH(ctx, name, *stateDir, sshArgs, os.Stdin, cmd.OutOrStdout(), cmd.OutOrStderr())
		},
	}

	return cmd
}

func runSSH(ctx context.Context, name string, stateDir string, sshArgs []string, in io.Reader, outW, errW io.Writer) error {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return errors.Wrap(err, "error looking up ssh client location")
	}

	dir := filepath.Join(stateDir, name)
	s, err := readState(dir)
	if err != nil {
		if strongerrors.IsNotFound(err) {
			return clusterNotFound(name)
		}
		return err
	}

	identifyFile := s.SSHIdentityFile
	if identifyFile == "" {
		maybe := filepath.Join(dir, "_output", "azureuser_rsa")
		if _, err := os.Stat(maybe); err == nil {
			identifyFile = maybe
		}
	}

	var args []string
	if len(identifyFile) > 0 {
		args = []string{"-i", identifyFile}
	}
	if len(sshArgs) > 0 {
		args = append(args, sshArgs...)
	}

	user := "azureuser"
	model, err := readAPIModel(dir)
	if err == nil {
		user = model.Properties.LinuxProfile.AdminUsername
	}

	args = append(args, user+"@"+makeFQDN(s))
	cmd := exec.CommandContext(ctx, ssh, args...)

	cmd.Stdout = outW
	cmd.Stderr = errW
	cmd.Stdin = in

	return cmd.Run()
}
