package cmd

import (
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphotobackup/internal"
	"github.com/ttomsu/gphotobackup/internal/backup"
	"time"
)

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.Flags().String("albumID", "", "")
	_ = viper.BindPFlag("albumID", backupCmd.Flags().Lookup("albumID"))
	backupCmd.Flags().Int("sinceDays", 0, "")
	_ = viper.BindPFlag("", backupCmd.Flags().Lookup("sinceDays"))
	backupCmd.Flags().String("start", "", "")
	_ = viper.BindPFlag("", backupCmd.Flags().Lookup("start"))
	backupCmd.Flags().String("end", "", "")
	_ = viper.BindPFlag("", backupCmd.Flags().Lookup("end"))
	backupCmd.Flags().String("out", ".", "")
	_ = viper.BindPFlag("", backupCmd.Flags().Lookup("out"))
	backupCmd.Flags().Int("workers", 3, "Concurrent download workers")
	_ = viper.BindPFlag("", backupCmd.Flags().Lookup("workers"))
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
		case viper.GetString("start") != "":
			start, err := time.Parse("2006-01-02", viper.GetString("start"))
			if err != nil {
				return errors.Wrap(err, "invalid --start")
			}
			sy, sm, sd := start.Date()

			ey, em, ed := time.Now().Date()
			if toStr := viper.GetString("end"); toStr != "" {
				if end, err := time.Parse("2006-01-02", toStr); err != nil {
					return errors.Wrap(err, "invalid --end")
				} else {
					ey, em, ed = end.Date()
				}

			}
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
		default:
			return errors.New("Must specify either --albumID, --sinceDays or --start[/--end]")
		}

		bs.Start(searchReq)
		return nil
	},
}
