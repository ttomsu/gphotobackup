package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphoto-sync/internal"
)

func init() {
	printCmd.AddCommand(itemCmd)

	//itemCmd.Flags().String("id", "", "Item ID")
	//itemCmd.MarkFlagRequired("id")
	//_ = viper.BindPFlags(itemCmd.Flags())
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
