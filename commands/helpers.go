package commands

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

func boolPtr(b bool) *bool {
	return &b
}

func generateRandom() (string, error) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	buf := make([]byte, 8)
	if _, err := r.Read(buf); err != nil {
		return "", errors.Wrap(err, "error generating random data")
	}
	return hex.EncodeToString(buf), nil
}

func makeFQDN(cfg state) string {
	if cfg.DNSPrefix == "" || cfg.Location == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s.cloudapp.azure.com", cfg.DNSPrefix, cfg.Location)
}

func readAPIModel(dir string) (apiModel, error) {
	var model apiModel
	data, err := ioutil.ReadFile(filepath.Join(dir, "apimodel.json"))
	if err != nil {
		return model, errors.Wrap(err, "error reading api model")
	}
	if err := json.Unmarshal(data, &model); err != nil {
		return model, errors.Wrap(err, "error unmarshaling api model")
	}
	return model, nil
}
