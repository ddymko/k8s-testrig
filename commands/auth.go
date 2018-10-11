package commands

import (
	"os"
	"path/filepath"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
	ini "gopkg.in/ini.v1"
)

func getSubFromAzDir(root string) (string, error) {
	subConfig, err := ini.Load(filepath.Join(root, "clouds.config"))
	if err != nil {
		return "", errors.Wrap(err, "error decoding cloud subscription config")
	}

	cloudConfig, err := ini.Load(filepath.Join(root, "config"))
	if err != nil {
		return "", errors.Wrap(err, "error decoding cloud config")
	}

	cloud := getSelectedCloudFromAzConfig(cloudConfig)
	return getCloudSubFromAzConfig(cloud, subConfig)
}

func getSelectedCloudFromAzConfig(f *ini.File) string {
	selectedCloud := "AzureCloud"
	if cloud, err := f.GetSection("cloud"); err == nil {
		if name, err := cloud.GetKey("name"); err == nil {
			if s := name.String(); s != "" {
				selectedCloud = s
			}
		}
	}
	return selectedCloud
}

func getCloudSubFromAzConfig(cloud string, f *ini.File) (string, error) {
	cfg, err := f.GetSection(cloud)
	if err != nil {
		return "", errors.New("could not find user defined subscription id")
	}
	sub, err := cfg.GetKey("subscription")
	if err != nil {
		return "", errors.Wrap(err, "error reading subscription id from cloud config")
	}
	return sub.String(), nil
}

func getAuthorizer() (autorest.Authorizer, error) {
	if os.Getenv("AZURE_AUTH_LOCATION") != "" {
		authorizer, err := auth.NewAuthorizerFromFile(azure.PublicCloud.ResourceManagerEndpoint)
		if err != nil {
			return nil, errors.Wrap(err, "error reading auth file")
		}
		return authorizer, nil
	}

	authorizer, err := auth.NewAuthorizerFromCLI()
	if err != nil {
		return nil, errors.New("could not get authorizer from azure CLI or environment")
	}
	return authorizer, nil
}
