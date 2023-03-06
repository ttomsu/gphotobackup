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
	backupCmd.Flags().Int("sinceDays", 0, "")
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

		searchReq := &photoslibrary.SearchMediaItemsRequest{}
		switch {
		case viper.GetString("albumID") != "":
			searchReq.AlbumId = viper.GetString("albumID")
		case viper.GetDuration("sinceDays") != 0:
			durDays := viper.GetDuration("sinceDays")
			ey, em, ed := time.Now().Date()
			sy, sm, sd := time.Now().Add(-1 * 24 * time.Hour * durDays).Date()
			searchReq.Filters = &photoslibrary.Filters{
				DateFilter: &photoslibrary.DateFilter{
					Ranges: []*photoslibrary.DateRange{
						{
							StartDate: &photoslibrary.Date{
								Day:   int64(sd),
								Month: int64(sm),
								Year:  int64(sy),
							},
							EndDate: &photoslibrary.Date{
								Day:   int64(ed),
								Month: int64(em),
								Year:  int64(ey),
							},
						},
					},
				},
			}
		}

		bs.Start(searchReq)
		return nil
	},
}
