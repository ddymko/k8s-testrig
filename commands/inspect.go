package commands

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/cpuguy83/strongerrors"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Inspect runs the command to inspect a cluster
func Inspect(ctx context.Context, stateDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Get details about an existing cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(ctx, *stateDir, args[0], cmd.OutOrStdout(), cmd.OutOrStderr())
		},
	}

	return cmd
}

type inspectItem struct {
	State state
	Model apiModel
}

func runInspect(ctx context.Context, stateDir, name string, outW, errW io.Writer) error {
	dir := filepath.Join(stateDir, name)
	var errs []error
	var inspect inspectItem

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return strongerrors.NotFound(errors.New("no such cluster"))
	}

	model, err := readAPIModel(dir)
	if err != nil {
		errs = append(errs, err)
	}
	inspect.Model = model

	s, err := readState(dir)
	if err != nil {
		errs = append(errs, err)
	}
	inspect.State = s

	data, err := json.MarshalIndent(inspect, "", "\t")
	if err != nil {
		return errors.Wrap(err, "error marshaling final output")
	}
	outW.Write(data)
	io.WriteString(outW, "\n")

	for _, e := range errs {
		io.WriteString(errW, e.Error()+"\n")
	}
	return nil
}
