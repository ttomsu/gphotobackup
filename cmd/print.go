package cmd

import (
	"context"
	"fmt"
	gphotos "github.com/gphotosuploader/google-photos-api-client-go/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/ttomsu/gphoto-sync/internal"
	"net/http"
	"net/http/httputil"
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

		cl, err := gphotos.NewClient(client)
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

func dumpingClient(client *http.Client) *http.Client {
	dt := &dumpingTransport{
		client.Transport,
	}
	client.Transport = dt
	return client
}

type dumpingTransport struct {
	http.RoundTripper
}

func (d dumpingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	dReq, _ := httputil.DumpRequest(request, false)
	fmt.Printf("Dumped Request:\n%v\n", string(dReq))
	resp, err := d.RoundTrip(request)
	if resp != nil {
		dRes, _ := httputil.DumpResponse(resp, false)
		fmt.Printf("Dumped Response:\n%v\n", string(dRes))
	}
	return resp, err
}
