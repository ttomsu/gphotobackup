package cmd

import (
	"context"
	"fmt"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/ttomsu/gphotobackup/internal"
)

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "",
	Long:  "",
	RunE: func(_ *cobra.Command, args []string) error {
		client, err := internal.NewClient()
		if err != nil {
			return errors.Wrapf(err, "new client")
		}

		svc, err := photoslibrary.New(client)
		if err != nil {
			return err
		}

		err = svc.Albums.List().Pages(context.Background(), func(resp *photoslibrary.ListAlbumsResponse) error {
			for _, album := range resp.Albums {
				fmt.Printf("Album found: %v, size %v (ID: %v)\n", album.Title, album.TotalMediaItems, album.Id)
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(printCmd)
}
