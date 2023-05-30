package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphotobackup/internal"
)

func init() {
	printCmd.AddCommand(itemCmd)

	itemCmd.PersistentFlags().String("id", "", "Item ID")
	itemCmd.MarkFlagRequired("id")
	checkError(viper.BindPFlags(itemCmd.PersistentFlags()))
}

var itemCmd = &cobra.Command{
	Use: "item",
	RunE: func(_ *cobra.Command, args []string) error {
		client, err := internal.NewClient()
		if err != nil {
			return errors.Wrapf(err, "new client")
		}

		cl, err := photoslibrary.New(client)
		if err != nil {
			return err
		}

		item, err := cl.MediaItems.Get(viper.GetString("id")).Do()
		if err != nil {
			return err
		}

		itemJSON, _ := json.MarshalIndent(item, "", "\t")
		fmt.Printf("Item found: %v\n", string(itemJSON))

		return nil
	},
}
