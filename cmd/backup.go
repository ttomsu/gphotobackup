package cmd

import (
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphoto-sync/internal"
	"github.com/ttomsu/gphoto-sync/internal/backup"
	"time"
)

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.Flags().String("albumID", "", "")
	backupCmd.Flags().Duration("since", 30*(24*time.Hour), "")
	backupCmd.Flags().String("out", ".", "")
	backupCmd.Flags().Int("workers", 3, "Concurrent download workers")

	_ = viper.BindPFlags(backupCmd.Flags())
}

var backupCmd = &cobra.Command{
	Use: "backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := internal.NewClient()
		if err != nil {
			return errors.Wrapf(err, "new client")
		}

		bs, err := backup.NewSession(client, viper.GetString("out"), viper.GetInt("workers"))
		if err != nil {
			return errors.Wrapf(err, "new session")
		}

		searchReq := &photoslibrary.SearchMediaItemsRequest{
			AlbumId: viper.GetString("albumID"),
		}
		bs.Start(searchReq)
		return nil
	},
}
