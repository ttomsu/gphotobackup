package backup

import (
	"context"
	"github.com/ttomsu/gphotobackup/internal/utils"
	"go.uber.org/zap"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
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
	bs.logger.Infof("~~~ Starting to backup recent photos...")
	bs.startInternal(searchReq, "", nil)
}

func (bs *Session) StartAlbums() {
	bs.logger.Info("~~~ Starting to back up albums...")
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
	bs.logger.Info("~~~ Starting to back up favorites...")
	dirName := "favorites"
	existingFiles := bs.existingFiles(dirName)
	searchReq := &photoslibrary.SearchMediaItemsRequest{
		PageSize: 100,
		Filters: &photoslibrary.Filters{
			IncludeArchivedMedia: true,
			FeatureFilter: &photoslibrary.FeatureFilter{
				IncludedFeatures: []string{
					"FAVORITES",
				},
			},
		},
	}

	bs.startInternal(searchReq, dirName, existingFiles)
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
	_ = f.Close()
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
