package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	gphotos "github.com/gphotosuploader/google-photos-api-client-go/v2"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "",
	Long:  "",
	RunE: func(_ *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		gphotosyncPath := filepath.Join(homeDir, ".config", "gphotosync")
		tokenFilepath := filepath.Join(gphotosyncPath, "token.json")
		jsonToken, err := os.ReadFile(tokenFilepath)
		if err != nil {
			return err
		}

		token := &oauth2.Token{}
		if err := json.Unmarshal(jsonToken, token); err != nil {
			return err
		}

		jsonData, err := os.ReadFile(filepath.Join(gphotosyncPath, "service_account.json"))
		if err != nil {
			return err
		}

		cfg := &oauth2.Config{}
		if err := json.Unmarshal(jsonData, cfg); err != nil {
			return err
		}

		cl, err := gphotos.NewClient(cfg.Client(context.Background(), token))
		if err != nil {
			return err
		}
		albums, err := cl.Albums.List(context.Background())
		if err != nil {
			return err
		}
		for _, album := range albums {
			fmt.Printf("Album found: %v, size %v\n", album.Title, album.MediaItemsCount)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(printCmd)
}
