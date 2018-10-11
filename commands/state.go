package commands

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cpuguy83/strongerrors"

	"github.com/pkg/errors"
)

type status string

var (
	stateInitialized status = "initialized"
	stateCreating    status = "creating"
	stateReady       status = "ready"
	stateFailure     status = "failed"
	stateRemoving    status = "removing"
)

type state struct {
	Location        string
	ResourceGroup   string
	DNSPrefix       string
	Status          status
	FailureMessage  string
	SSHIdentityFile string
	DeploymentName  string
	CreatedAt       time.Time
}

func writeState(dir string, s state) error {
	filePath := filepath.Join(dir, "state.json")
	stateJSON, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		return errors.Wrap(err, "error marshaling state")
	}
	return errors.Wrap(ioutil.WriteFile(filePath, stateJSON, 0644), "error writing state file")
}

func readState(dir string) (state, error) {
	data, err := ioutil.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return state{}, strongerrors.NotFound(err)
		}
		return state{}, errors.Wrap(err, "error reading state file")
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return s, errors.Wrap(err, "error unmarshaling state data")
	}
	return s, nil
}
