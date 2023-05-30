package backup

import (
	"context"
	"fmt"
	"github.com/ttomsu/gphotobackup/internal/utils"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Session struct {
	svc         *photoslibrary.Service
	queue       chan *mediaItemWrapper
	wg          *sync.WaitGroup
	baseDestDir string
	workers     []*worker
	logger      *zap.SugaredLogger
}

func NewSession(client *http.Client, baseDestDir string, workerCount int, logger *zap.SugaredLogger) (*Session, error) {
	svc, err := photoslibrary.New(client)
	if err != nil {
		return nil, err
	}

	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	workers := make([]*worker, workerCount)
	for i := 0; i < workerCount; i++ {
		workers[i] = &worker{
			id:     i,
			stop:   make(chan bool),
			wg:     wg,
			mu:     mu,
			client: client,
			logger: logger,
		}
	}

	logger.Infoln("Starting new backup session...")
	return &Session{
		svc:         svc,
		queue:       make(chan *mediaItemWrapper, 100),
		wg:          wg,
		baseDestDir: baseDestDir,
		workers:     workers,
		logger:      logger,
	}, nil
}

func (bs *Session) Start(searchReq *photoslibrary.SearchMediaItemsRequest) {
	bs.startInternal(searchReq, "", nil)
}

func (bs *Session) startInternal(searchReq *photoslibrary.SearchMediaItemsRequest, destDir string, existingFiles map[string]bool) {
	for _, w := range bs.workers {
		go w.start(bs.queue)
	}
	defer bs.Stop()

	totalCount := 0
	err := bs.svc.MediaItems.Search(searchReq).
		Pages(context.Background(), func(resp *photoslibrary.SearchMediaItemsResponse) error {
			count := len(resp.MediaItems)
			bs.wg.Add(count)
			totalCount = totalCount + count
			bs.logger.Infof("Adding %v items to queue (%v)", count, totalCount)

			for _, item := range resp.MediaItems {
				miw := bs.wrap(item, destDir)
				if existingFiles != nil {
					fullFilename := miw.filename(false)
					isDup, _ := existingFiles[fullFilename]
					if isDup {
						bs.logger.Warnf("Duplicate found, filename: %v", fullFilename)
					} else {
						existingFiles[fullFilename] = true
					}
				}
				bs.queue <- miw
			}
			return nil
		})
	if err != nil {
		bs.logger.Errorf("Search error: %v", err)
	}
	bs.wg.Wait()
}

func (bs *Session) StartAlbums() {
	bs.logger.Info("Starting to back up albums")
	err := bs.svc.Albums.List().Pages(context.Background(), func(resp *photoslibrary.ListAlbumsResponse) error {
		for _, album := range resp.Albums {
			albumPath := filepath.Join("albums", utils.Sanitize(album.Title))
			existingFiles := bs.existingFiles(albumPath)

			if len(existingFiles) == int(album.TotalMediaItems) {
				bs.logger.Infof("Album \"%v\" already contains %v items, skipping", albumPath, len(existingFiles))
				continue
			}

			bs.logger.Infof("Backing up %v items (have %v) from album to %v", album.TotalMediaItems, len(existingFiles), albumPath)
			searchReq := &photoslibrary.SearchMediaItemsRequest{
				PageSize: 100,
				AlbumId:  album.Id,
			}
			bs.startInternal(searchReq, albumPath, existingFiles)

			for filename, inAlbum := range existingFiles {
				if !inAlbum {
					bs.logger.Warnf("Extra file found: %v", filepath.Join(albumPath, filename))
				}
			}
		}
		return nil
	})
	if err != nil {
		bs.logger.Errorf("Albums error: %v", err)
	}
}

func (bs *Session) StartFavorites() {
	dirName := "favorites"
	existingFiles := bs.existingFiles(dirName)
	searchReq := &photoslibrary.SearchMediaItemsRequest{
		PageSize: 100,
		Filters: &photoslibrary.Filters{
			FeatureFilter: &photoslibrary.FeatureFilter{
				IncludedFeatures: []string{
					"FAVORITES",
				},
			},
		},
	}
	bs.logger.Info("Starting to back up favorites")
	bs.startInternal(searchReq, dirName, existingFiles)
}

func (bs *Session) Stop() {
	bs.wg.Add(len(bs.workers))
	for _, w := range bs.workers {
		w.stop <- true
	}
	bs.wg.Wait()
}

func (bs *Session) existingFiles(dir string) map[string]bool {
	m := make(map[string]bool)
	fullDir := filepath.Join(bs.baseDestDir, dir)
	f, err := os.Open(fullDir)
	if err != nil {
		bs.logger.Errorf("error opening fullDir %v to count: %v", fullDir, err)
		return m
	}
	list, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		bs.logger.Errorf("error reading dirnames: %v", err)
		return m
	}
	m = make(map[string]bool, len(list))
	for _, filename := range list {
		m[filename] = false
	}
	return m
}

type worker struct {
	id     int
	stop   chan bool
	wg     *sync.WaitGroup
	mu     *sync.Mutex
	client *http.Client
	logger *zap.SugaredLogger
}

func (w *worker) start(queue <-chan *mediaItemWrapper) {
	for {
		select {
		case miw := <-queue:
			if viper.GetBool("verbose") {
				w.logger.Debugf("Worker %v got %v of size %vw x %vh created at %v", w.id, miw.src.MimeType, miw.src.MediaMetadata.Width, miw.src.MediaMetadata.Height, miw.src.MediaMetadata.CreationTime)
			}

			err := w.ensureDestExists(miw)
			if err != nil {
				w.logger.Errorf("Error creating dest for %v, err: %v", miw.src.Filename, err)
				w.wg.Done()
				continue
			}
			if !w.fileExists(miw.destFilepath()) {
				data, err := w.fetchItem(miw)
				if err != nil {
					w.logger.Errorf("Error fetching %v, err: %v", miw.src.Filename, err)
					w.wg.Done()
					continue
				}
				if err = w.writeItem(miw, data); err != nil {
					w.logger.Errorf("Error writing %v, err: %v", miw.destFilepath(), err)
					w.wg.Done()
					continue
				}
			} else {
				if viper.GetBool("verbose") {
					w.logger.Debugf("%v already exists", miw.destFilepathShort())
				}
			}
			w.wg.Done()
		case <-w.stop:
			w.logger.Debugf("Worker %v received stop signal", w.id)
			w.wg.Done()
			return
		}
	}
}

func (w *worker) ensureDestExists(miw *mediaItemWrapper) error {
	w.mu.Lock()
	err := os.MkdirAll(miw.destDir(), 0755)
	w.mu.Unlock()
	return errors.Wrapf(err, "error creating dest dir %v", miw.destDir())
}

func (w *worker) fileExists(destFilename string) bool {
	_, err := os.Stat(destFilename)
	return err == nil
}

func (w *worker) fetchItem(miw *mediaItemWrapper) ([]byte, error) {
	var url string
	switch {
	case miw.src.MediaMetadata.Video != nil:
		if miw.src.MediaMetadata.Video.Status != "READY" {
			return []byte{}, errors.Errorf("video %v is not yet processed", miw.src.Filename)
		}
		url = fmt.Sprintf("%v=dv", miw.src.BaseUrl)
	case miw.src.MediaMetadata.Photo != nil:
		url = fmt.Sprintf("%v=d", miw.src.BaseUrl)
	}

	if resp, err := w.client.Get(url); err != nil {
		return []byte{}, errors.Wrapf(err, "error fetching data for %v", miw.src.Filename)
	} else if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return []byte{}, errors.Errorf("Non 200 status returned for URL %v, body: %v", url, string(body))
	} else {
		return io.ReadAll(resp.Body)
	}
}

func (w *worker) writeItem(miw *mediaItemWrapper, data []byte) error {
	defer func() {
		w.logger.Debugf("Worker %v finished %v in %v", w.id, miw.destFilepathShort(), time.Since(miw.startTime))
	}()
	if err := os.WriteFile(miw.destFilepath(), data, 0644); err != nil {
		return errors.Wrapf(err, "writing item %v", miw.src.Id)
	}
	if miw.src.MediaMetadata.CreationTime != "" && !miw.creationTime.IsZero() {
		return errors.Wrap(os.Chtimes(miw.destFilepath(), miw.creationTime, miw.creationTime), "error changing times")
	}
	return nil
}

type mediaItemWrapper struct {
	src          *photoslibrary.MediaItem
	baseDestDir  string
	creationTime time.Time
	startTime    time.Time
	destDirName  string
}

func (bs *Session) wrap(mi *photoslibrary.MediaItem, destDirName string) *mediaItemWrapper {
	t, err := time.Parse(time.RFC3339, mi.MediaMetadata.CreationTime)
	if err != nil {
		bs.logger.Errorf("Error parsing timestamp %v for id %v", mi.MediaMetadata.CreationTime, mi.Id)
	}
	return &mediaItemWrapper{
		src:          mi,
		baseDestDir:  bs.baseDestDir,
		creationTime: t,
		startTime:    time.Now(),
		destDirName:  destDirName,
	}
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
