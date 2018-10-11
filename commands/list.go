package commands

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// List returns a command to list the existing clusters.
// Note that this lists from the local state, which may differ from state in Azure.
func List(ctx context.Context, stateDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List available clusters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(ctx, *stateDir, cmd.OutOrStdout(), cmd.OutOrStderr())
		},
	}

	return cmd
}

var header = []byte("NAME\tSTATUS\tFQDN\n")

type listItem struct {
	Name   string
	Status status
	FQDN   string
}

func runList(ctx context.Context, stateDir string, outW, errW io.Writer) error {
	buf := bytes.NewBuffer(nil)
	tw := tabwriter.NewWriter(buf, 20, 1, 3, ' ', tabwriter.TabIndent)

	if _, err := tw.Write(header); err != nil {
		return errors.Wrap(err, "error writing table header4")
	}

	ls, err := ioutil.ReadDir(stateDir)
	if err != nil {
		return errors.Wrapf(err, "error reading state dir %q", stateDir)
	}

	var items []listItem
	var errs []error
	for _, e := range ls {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".removing") {
			continue
		}

		dir := filepath.Join(stateDir, e.Name())

		s, err := readState(dir)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "error reading state for %q", e.Name()))
		}

		items = append(items, listItem{
			Name:   e.Name(),
			Status: s.Status,
			FQDN:   makeFQDN(s),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	for _, i := range items {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		io.WriteString(tw, i.Name+"\t")
		io.WriteString(tw, strings.Title(string(i.Status))+"\t")
		io.WriteString(tw, i.FQDN)
		io.WriteString(tw, "\n")
	}

	if err := tw.Flush(); err != nil {
		return errors.Wrap(err, "error flushing table writer")
	}

	for i, err := range errs {
		io.WriteString(errW, err.Error()+"\n")
		if i == len(errs)-1 {
			io.WriteString(errW, "\n")
		}
	}

	io.Copy(outW, buf)

	return nil
}
