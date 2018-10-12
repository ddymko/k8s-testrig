package commands

import (
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/cpuguy83/strongerrors"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
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

func (s *sshConfig) Set(p string) error {
	if len(s.PublicKeys) >= 1 {
		return errors.New("only one ssh key is supported")
	}
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return errors.Wrap(err, "error reading ssh key data")
	}
	s.PublicKeys = append(s.PublicKeys, sshKey{KeyData: string(b)})
	return nil
}

func (s *sshConfig) String() string {
	var b strings.Builder
	for i, k := range s.PublicKeys {
		data := k.KeyData
		if i < len(s.PublicKeys)-1 {
			data += "\n\n"
		}
		b.WriteString(data)
	}
	return b.String()
}

func (s *sshConfig) Type() string {
	return "sshKey"
}

func createSSHKey(ctx context.Context, keyW io.Writer) (string, error) {
	privateKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return "", errors.Wrap(err, "error generating encrytption key")
	}

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(keyW, privateKeyPEM); err != nil {
		return "", errors.Wrap(err, "error encoding private key")
	}

	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", errors.Wrap(err, "error creating ssh public key")
	}

	return string(ssh.MarshalAuthorizedKey(pub)), nil
}

func readACSDeployment(dir string) (interface{}, interface{}, error) {
	template := make(map[string]interface{})
	deployB, err := ioutil.ReadFile(filepath.Join(dir, "_output", "azuredeploy.json"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "error reading deployment template")
	}
	if err := json.Unmarshal(deployB, &template); err != nil {
		return nil, nil, errors.Wrap(err, "error unmarshaling deployment template")
	}

	params := make(map[string]interface{})
	paramsB, err := ioutil.ReadFile(filepath.Join(dir, "_output", "azuredeploy.parameters.json"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "error reading deployment parameters")
	}
	if err := json.Unmarshal(paramsB, &params); err != nil {
		return nil, nil, errors.Wrap(err, "error unmarshaling deployment template")
	}

	return template, params["parameters"], nil
}

func clusterNotFound(name string) error {
	return strongerrors.NotFound(errors.Errorf("no such cluster: %q", name))
}

func isAzureNotFound(err error) bool {
	if err == nil {
		return false
	}

	switch e := err.(type) {
	case *azure.RequestError:
		if e.StatusCode != 0 {
			return e.StatusCode == http.StatusNotFound
		}
		return isAzureNotFound(e.Original)
	case autorest.DetailedError:
		if e.StatusCode != 0 {
			return e.StatusCode == http.StatusNotFound
		}
		return isAzureNotFound(e.Original)
	}

	return false
}