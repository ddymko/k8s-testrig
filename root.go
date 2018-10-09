package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cpuguy83/testrig/commands"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func main() {
	var stateDir string
	ctx, cancel := context.WithCancel(context.Background())

	cmd := &cobra.Command{
		Use:           filepath.Base(os.Args[0]),
		Short:         "Quickly create and manage test Kubernetes clusters on Azure for testing purposes",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if stateDir == "" {
				home, err := homedir.Dir()
				if err != nil {
					return errors.Wrap(err, "error determining home dir for local persistent state")
				}
				stateDir = filepath.Join(home, ".testrig")

			}

			chSig := make(chan os.Signal)
			signal.Notify(chSig, syscall.SIGTERM, syscall.SIGINT)
			go func() {
				<-chSig
				cancel()
			}()

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&stateDir, "state-dir", stateDir, "directory to store state information to")

	cmd.AddCommand(
		commands.Create(ctx, &stateDir),
		commands.List(ctx, &stateDir),
		commands.Inspect(ctx, &stateDir),
		commands.SSH(ctx, &stateDir),
		commands.KubeConfig(ctx, &stateDir),
		commands.Remove(ctx, &stateDir),
	)

	if err := cmd.Execute(); err != nil {
		io.WriteString(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}

}
