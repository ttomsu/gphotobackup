package cmd

import (
	"context"
	"fmt"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphotobackup/internal"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().String("creds", "", "Path to the OAuth 2.0 credentials file for the Photos Library API")
	loginCmd.MarkFlagRequired("creds")
	_ = viper.BindPFlag("creds", loginCmd.Flags().Lookup("creds"))

	loginCmd.Flags().Bool("browser", true, "Use a local browser to obtain user consent.")
	_ = viper.BindPFlag("browser", loginCmd.Flags().Lookup("browser"))
}

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Do the OAuth dance with the specified credentials and cache the results",
	RunE: func(_ *cobra.Command, args []string) error {
		oauthCreds, err := os.ReadFile(viper.GetString("creds"))
		if err != nil {
			return errors.Wrapf(err, "reading --creds flag")
		}
		cfg, err := google.ConfigFromJSON(oauthCreds, photoslibrary.PhotoslibraryReadonlyScope)
		if err != nil {
			return errors.Wrapf(err, "cfg from creds data")
		}
		cfg.RedirectURL = "http://localhost:9898/login"

		url := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
		codeChan := make(chan string, 1)
		var server *http.Server

		if viper.GetBool("browser") {
			err = openInBrowser(url)
			if err != nil {
				return errors.Wrapf(err, "auth code url")
			}

			server := &http.Server{Addr: ":9898"}
			http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
				codeChan <- r.URL.Query().Get("code")
				w.Write([]byte("You can close this window now!"))
			})
			go server.ListenAndServe()

			fmt.Println("Waiting for approval...")
		} else {
			fmt.Printf("In another browser, navigate to:\n%v\n\n", url)
			fmt.Println("Enter query parameter 'code' here:")
			var c string
			if _, err := fmt.Scan(&c); err != nil {
				return errors.Wrap(err, "getting code from command line")
			} else if c == "" {
				return errors.New("code cannot be blank")
			}
			codeChan <- c
		}

		timeout := time.NewTimer(2 * time.Minute)
		var code string
		select {
		case code = <-codeChan:
		case <-timeout.C:
		}
		if server != nil {
			_ = server.Shutdown(context.Background())
		}
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
