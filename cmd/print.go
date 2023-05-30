package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphotobackup/internal/utils"
	"os"
	"sort"
	"strings"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/ttomsu/gphotobackup/internal"
)

func init() {
	rootCmd.AddCommand(printCmd)

	printCmd.PersistentFlags().String("out", "", "")
	_ = viper.BindPFlags(printCmd.PersistentFlags())
	//_ = viper.BindPFlag("out", printCmd.PersistentFlags().Lookup("out"))
}

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Print all albums available",
	RunE: func(_ *cobra.Command, args []string) error {
		client, err := internal.NewClient()
		if err != nil {
			return errors.Wrapf(err, "new client")
		}

		fmt.Printf("~~~ Outdir: %v\n", viper.GetString("out"))

		svc, err := photoslibrary.New(client)
		if err != nil {
			return errors.Wrap(err, "service client")
		}

		details := make([]*albumDetail, 0, 256)
		total := 0
		err = svc.Albums.List().Pages(context.Background(), func(resp *photoslibrary.ListAlbumsResponse) error {
			total = total + len(resp.Albums)
			fmt.Printf("Album count: %v\n", total)
			for _, album := range resp.Albums {
				details = append(details, &albumDetail{
					Name: utils.Sanitize(album.Title),
					Id:   album.Id,
					Size: int(album.TotalMediaItems),
				})
			}
			return nil
		})
		if err != nil {
			return errors.Wrap(err, "album list pages")
		}
		sort.SliceStable(details, func(i, j int) bool {
			return strings.ToLower(details[i].Name) < strings.ToLower(details[j].Name)
		})

		var f *os.File
		defer f.Close()
		if out := viper.GetString("out"); out == "" {
			f = os.Stdout
		} else {
			f, err = os.OpenFile(out, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return errors.Wrap(err, "opening out file")
			}
		}

		writer := bufio.NewWriter(f)
		for _, detail := range details {
			b, _ := json.Marshal(detail)
			_, err = writer.WriteString(string(b) + "\n")
			if err != nil {
				return errors.Wrap(err, "writing details")
			}
		}
		return writer.Flush()
	},
}

type albumDetail struct {
	Name string `json:"name,omitempty"`
	Id   string `json:"id,omitempty"`
	Size int    `json:"size,omitempty"`
}
