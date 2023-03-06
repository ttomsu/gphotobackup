package internal

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func NewClient() (*http.Client, error) {
	jsonToken, err := ReadFromConfigDir(TokenFilename)
	if err != nil {
		return nil, errors.Wrap(err, "reading token.json. Retry after 'gphotosync login'")
	} else if len(jsonToken) == 0 {
		return nil, errors.New("invalid token.json. Retry after running 'gphotosync login'?")
	}
	token := &oauth2.Token{}
	if err = json.Unmarshal(jsonToken, token); err != nil {
		return nil, errors.Wrap(err, "unmarshalling token")
	}

	oauthClientData, err := ReadFromConfigDir(OAuthClientFilename)
	if err != nil {
		return nil, errors.Wrap(err, "reading oauth_client.json. Retry after 'gphotosync login'")
	} else if len(oauthClientData) == 0 {
		return nil, errors.New("invalid oauth_client data. Retry after 'gphotosync login'")
	}

	cfg, err := google.ConfigFromJSON(oauthClientData, photoslibrary.PhotoslibraryReadonlyScope)
	if err != nil {
		return nil, errors.Wrap(err, "oauth client cfg")
	}

	return cfg.Client(context.Background(), token), nil
}
