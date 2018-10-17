package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Azure/k8s-testrig/commands"
	"github.com/cpuguy83/strongerrors"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func main() {
	var (
		stateDir   string
		configFile string
		err        error
	)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := &cobra.Command{
		Use:           filepath.Base(os.Args[0]),
		Short:         "Quickly create and manage test Kubernetes clusters on Azure",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if stateDir == "" {
				if err != nil {
					return err
				}
				return errors.New("state dir not set")
			}
			if err != nil {
				return err
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

	stateDir, err = defaultStateDir()
	configFile = defaultConifgPath(stateDir)

	flags := cmd.PersistentFlags()
	flags.StringVar(&stateDir, "state-dir", stateDir, "Directory to store state information to")
	flags.StringVar(&configFile, "config", configFile, "Location of user config file")

	if len(os.Args) > 1 {
		err = flags.Parse(os.Args[1:])
	}

	var cfg commands.UserConfig
	cfg, err = commands.ReadUserConfig(configFile)
	if err != nil && strongerrors.IsNotFound(err) && configFile == defaultConifgPath(stateDir) {
		err = nil
	}

	cmd.AddCommand(
		commands.Create(ctx, stateDir, &cfg),
		commands.List(ctx, stateDir),
		commands.Inspect(ctx, stateDir),
		commands.SSH(ctx, stateDir),
		commands.KubeConfig(ctx, stateDir),
		commands.Remove(ctx, stateDir, &cfg),
	)

	if err := cmd.Execute(); err != nil {
		io.WriteString(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}

}

func defaultStateDir() (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return "", errors.Wrap(err, "state dir not provided and could not determine default location based on user home dir")
	}
	return filepath.Join(homeDir, ".testrig"), nil
}

func defaultConifgPath(stateDir string) string {
	if stateDir == "" {
		return ""
	}
	return filepath.Join(stateDir, "config.toml")
}
