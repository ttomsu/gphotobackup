package cmd

import (
	"context"
	"encoding/json"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphotobackup/internal"
)

func init() {
	printCmd.AddCommand(albumCmd)

	albumCmd.MarkFlagRequired("id")
	checkError(viper.BindPFlags(albumCmd.PersistentFlags()))
}

var albumCmd = &cobra.Command{
	Use:   "album",
	Short: "Print media items in this album",
	RunE: func(_ *cobra.Command, args []string) error {
		logger := NewLogger()
		client, err := internal.NewClient()
		if err != nil {
			return errors.Wrapf(err, "new client")
		}

		cl, err := photoslibrary.New(client)
		if err != nil {
			return err
		}

		albumID := viper.GetString("id")
		if albumID == "" {
			return errors.New("--id missing")
		}
		total := 0
		err = cl.MediaItems.Search(&photoslibrary.SearchMediaItemsRequest{AlbumId: albumID}).
			Pages(context.Background(), func(resp *photoslibrary.SearchMediaItemsResponse) error {
				total = total + len(resp.MediaItems)
				for _, item := range resp.MediaItems {
					itemJSON, _ := json.MarshalIndent(item, "", "\t")
					logger.Infof("Photo/video found: %v", string(itemJSON))
				}
				return nil
			})
		if err != nil {
			return err
		}
		logger.Infof("Total items found: %v", total)
		return nil
	},
}
