package internal

import (
	"encoding/json"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

var (
	userHome, _         = os.UserHomeDir()
	gPhotoSyncConfigDir = filepath.Join(userHome, ".config", "gphotosync")

	TokenFilename       = "token.json"
	OAuthClientFilename = "oauth_client.json"
)

func ReadFromConfigDir(filename string) ([]byte, error) {
	fp := filepath.Join(gPhotoSyncConfigDir, filename)
	return os.ReadFile(fp)
}

func WriteJSONToConfigDir(filename string, j any) error {
	d, err := json.MarshalIndent(j, "", "\t")
	if err != nil {
		return errors.Wrap(err, "writing json to config dir")
	}
	return WriteToConfigDir(filename, d)
}

func WriteToConfigDir(filename string, data []byte) error {
	if err := os.MkdirAll(gPhotoSyncConfigDir, 0700); err != nil {
		return errors.Wrap(err, "creating config dirs")
	}

	fp := filepath.Join(gPhotoSyncConfigDir, filename)
	if err := os.WriteFile(fp, data, 0600); err != nil {
		return errors.Wrapf(err, "writing config file %v", filename)
	}
	return nil
}
