package backup

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/ttomsu/gphotobackup/internal/utils"
)

type mediaItemWrapper struct {
	src          *photoslibrary.MediaItem
	baseDestDir  string
	creationTime time.Time
	startTime    time.Time
	destDirName  string
}

func (miw *mediaItemWrapper) destDir() string {
	dir := "unknown"
	if miw.destDirName != "" {
		dir = miw.destDirName
	} else if !miw.creationTime.IsZero() {
		dir = miw.creationTime.Local().Format("2006/01/02")
	}
	return filepath.Join(miw.baseDestDir, dir)
}

func (miw *mediaItemWrapper) destFilepath() string {
	return filepath.Join(miw.destDir(), miw.filename(false))
}

func (miw *mediaItemWrapper) destFilepathShort() string {
	return filepath.Join(miw.destDir(), miw.filename(true))
}

func (miw *mediaItemWrapper) filename(short bool) string {
	lastDotIndex := strings.LastIndex(miw.src.Filename, ".")
	var filename string
	if lastDotIndex > 0 {
		parts := []string{
			utils.Sanitize(miw.src.Filename[0:lastDotIndex]),
			miw.src.Filename[lastDotIndex+1 : len(miw.src.Filename)],
		}
		id := miw.src.Id
		if short && len(id) > 8 {
			id = fmt.Sprintf("...%v", id[len(id)-9:len(id)-1])
		}
		filename = fmt.Sprintf("%v-%v.%v", parts[0], id, parts[1])
	} else {
		filename = utils.Sanitize(miw.src.Filename)
	}
	return filename
}
