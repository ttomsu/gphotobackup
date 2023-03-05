package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphoto-sync/internal"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	gphotos "github.com/gphotosuploader/google-photos-api-client-go/v2"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "",
	Long:  "",
	RunE: func(_ *cobra.Command, args []string) error {
		oauthCreds, err := os.ReadFile(viper.GetString("creds"))
		if err != nil {
			return errors.Wrapf(err, "reading --creds flag")
		}
		cfg, err := google.ConfigFromJSON(oauthCreds, gphotos.PhotoslibraryReadonlyScope)
		if err != nil {
			return errors.Wrapf(err, "cfg from creds data")
		}
		cfg.RedirectURL = "http://localhost:9898/login"

		url := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
		err = openInBrowser(url)
		if err != nil {
			return errors.Wrapf(err, "auth code url")
		}

		codeChan := make(chan string, 1)
		server := &http.Server{Addr: ":9898"}
		http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
			codeChan <- r.URL.Query().Get("code")
			w.Write([]byte("You can close this window now!"))
		})
		go server.ListenAndServe()

		fmt.Println("Waiting for approval...")
		var code string
		timeout := time.NewTimer(2 * time.Minute)
		select {
		case code = <-codeChan:
		case <-timeout.C:
		}
		_ = server.Shutdown(context.Background())
		if code == "" {
			return errors.New("Did not get code in time.")
		}
		token, err := cfg.Exchange(context.Background(), code)
		if err != nil {
			return errors.Wrapf(err, "exchange")
		}

		if err := internal.WriteJSONToConfigDir(internal.TokenFilename, token); err != nil {
			return errors.Wrapf(err, "writing token.json")
		}
		if err := internal.WriteToConfigDir(internal.OAuthClientFilename, oauthCreds); err != nil {
			return errors.Wrapf(err, "writing oauth_client.json")
		}
		fmt.Println("Logged in!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().String("creds", "", "Path to the OAuth 2.0 credentials file for the Photos Library API")
	loginCmd.MarkFlagRequired("creds")
	_ = viper.BindPFlags(loginCmd.Flags())
}

func openInBrowser(url string) (err error) {
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return
}
