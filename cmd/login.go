package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
		jsonData, err := os.ReadFile("/Users/ttomsu/Downloads/credentials.json")
		if err != nil {
			return err
		}
		cfg, err := google.ConfigFromJSON(jsonData, gphotos.PhotoslibraryReadonlyScope)
		if err != nil {
			return err
		}
		url := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
		err = openInBrowser(url)
		if err != nil {
			return err
		}

		codeChan := make(chan string, 1)
		server := &http.Server{Addr: ":9898"}
		http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
			codeChan <- r.URL.Query().Get("code")
			w.Write([]byte("You can close this window now!"))
		})
		go server.ListenAndServe()

		fmt.Println("Blocking...")
		code := <-codeChan
		fmt.Println("Unblocked!")
		server.Shutdown(context.Background())
		token, err := cfg.Exchange(context.Background(), code)
		if err != nil {
			return err
		}

		tokenBytes, err := json.MarshalIndent(token, "", "\t")
		if err != nil {
			return err
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		credDir := filepath.Join(homeDir, ".config", "gphotosync")
		if err := os.MkdirAll(credDir, 0700); err != nil {
			return err
		}
		credPath := filepath.Join(credDir, "token.json")
		if err := os.WriteFile(credPath, tokenBytes, os.FileMode(0600)); err != nil {
			return err
		}
		fmt.Println("Logged in!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
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
