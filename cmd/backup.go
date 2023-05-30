package cmd

import (
	"fmt"
	"time"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttomsu/gphotobackup/internal"
	"github.com/ttomsu/gphotobackup/internal/backup"
)

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.PersistentFlags().String("albumID", "", "")
	backupCmd.PersistentFlags().Bool("albums", false, "Backup albums too")
	backupCmd.PersistentFlags().Bool("favorites", false, "Backup favorites too")
	backupCmd.PersistentFlags().Int("sinceDays", 0, "")
	backupCmd.PersistentFlags().String("start", "", "")
	backupCmd.PersistentFlags().String("end", "", "")
	backupCmd.PersistentFlags().String("out", ".", "")
	backupCmd.PersistentFlags().Int("workers", 3, "Concurrent download workers")
	backupCmd.PersistentFlags().Bool("verbose", true, "Emit details of all media items")

	checkError(viper.BindPFlags(backupCmd.PersistentFlags()))
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Download all photos/videos found in the specified album or date range",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := internal.NewClient()
		if err != nil {
			return errors.Wrapf(err, "new client")
		}

		fmt.Printf("~~~ Outdir: %v\n", viper.GetString("out"))
		bs, err := backup.NewSession(client, viper.GetString("out"), viper.GetInt("workers"))
		if err != nil {
			return errors.Wrapf(err, "new session")
		}

		searchReq := &photoslibrary.SearchMediaItemsRequest{
			PageSize: 100,
		}
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

		if viper.GetBool("favorites") {
			bs.StartFavorites()
		}

		if viper.GetBool("albums") {
			bs.StartAlbums()
		}

		return nil
	},
}
