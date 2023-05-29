package backup

import (
	"fmt"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"reflect"
	"testing"
)

func TestFilename(t *testing.T) {
	type test struct {
		input string
		want  string
	}

	tests := []test{
		{input: "foobar.jpg", want: "foobar-id012345678901234567890123456789id.jpg"},
		{input: "foobar.mo.jpg", want: "foobar_mo-id012345678901234567890123456789id.jpg"},
		{input: "1/2/2023 - foobar.jpg", want: "1_2_2023___foobar-id012345678901234567890123456789id.jpg"},
		{input: "some~special!chars@.jpg", want: "some_special_chars_-id012345678901234567890123456789id.jpg"},
		{input: "no-extension/onfile", want: "no_extension_onfile"},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			miw := &mediaItemWrapper{
				src: &photoslibrary.MediaItem{
					Filename: tc.input,
					Id:       "id012345678901234567890123456789id",
				},
			}
			got := miw.filename(false)
			if !reflect.DeepEqual(tc.want, got) {
				t.Fatalf("expected: %v, got: %v", tc.want, got)
			}
		})
	}
}
