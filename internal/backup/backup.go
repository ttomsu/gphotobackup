package backup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var (
	specialChars = regexp.MustCompile(`[^\w]`)
)

type Session struct {
	svc               *photoslibrary.Service
	queue             chan *mediaItemWrapper
	wg                *sync.WaitGroup
	baseDestDir       string
	workers           []*worker
	existingFilenames map[string]bool
	filenameChan      chan string
}

func NewSession(client *http.Client, baseDestDir string, workerCount int) (*Session, error) {
	svc, err := photoslibrary.New(client)
	if err != nil {
		return nil, err
	}

	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	workers := make([]*worker, workerCount)
	for i := 0; i < workerCount; i++ {
		workers[i] = &worker{
			id:          i,
			stop:        make(chan bool),
			wg:          wg,
			baseDestDir: baseDestDir,
			mu:          mu,
			client:      client,
		}
	}

	return &Session{
		svc:          svc,
		queue:        make(chan *mediaItemWrapper, 100),
		wg:           wg,
		baseDestDir:  baseDestDir,
		workers:      workers,
		filenameChan: make(chan string, 40000),
	}, nil
}

func (bs *Session) Start(searchReq *photoslibrary.SearchMediaItemsRequest, destDirName string) {
	for _, w := range bs.workers {
		go w.start(bs.queue)
	}
	defer bs.Stop()

	err := bs.svc.MediaItems.Search(searchReq).
		Pages(context.Background(), func(resp *photoslibrary.SearchMediaItemsResponse) error {
			bs.wg.Add(len(resp.MediaItems))
			fmt.Printf("Adding %v items to queue\n", len(resp.MediaItems))

			for _, item := range resp.MediaItems {
				miw := wrap(item, bs.baseDestDir, destDirName)
				bs.filenameChan <- miw.filename(false)
				bs.queue <- miw
			}
			return nil
		})
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
	}
	bs.wg.Wait()
}

func (bs *Session) StartAlbums() {
	err := bs.svc.Albums.List().Pages(context.Background(), func(resp *photoslibrary.ListAlbumsResponse) error {
		for _, album := range resp.Albums {
			albumPath := filepath.Join("albums", escapeAlbumTitle(album.Title))
			existingCount := bs.countFiles(albumPath)

			fmt.Printf("Album count: %v, dir count: %v\n", album.TotalMediaItems, existingCount)
			if existingCount == int(album.TotalMediaItems) {
				fmt.Printf("Album \"%v\" already contains %v items, skipping\n", albumPath, existingCount)
				continue
			}

			fmt.Printf("Backing up %v items from album to %v\n", album.TotalMediaItems, albumPath)
			searchReq := &photoslibrary.SearchMediaItemsRequest{
				PageSize: 100,
				AlbumId:  album.Id,
			}
			bs.Start(searchReq, albumPath)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Albums error: %v\n", err)
	}
}

func (bs *Session) StartFavorites() {
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
	fmt.Println("Starting to back up favorites")
	bs.Start(searchReq, "favorites")
}

func (bs *Session) Stop() {
	bs.wg.Add(len(bs.workers))
	for _, w := range bs.workers {
		w.stop <- true
	}
	bs.wg.Wait()

	if bs.existingFilenames != nil {
		for name := range bs.filenameChan {
			if _, ok := bs.existingFilenames[name]; ok {
				bs.existingFilenames[name] = true
			} else {
				fmt.Printf("Backup copy missing: %v\n", name)
			}
		}
		for k, v := range bs.existingFilenames {
			if !v {
				fmt.Printf("Extra backup file found: %v\n", k)
			}
		}
	}
}

func (bs *Session) countFiles(dir string) int {
	bs.existingFilenames = make(map[string]bool, 1)
	fullDir := filepath.Join(bs.baseDestDir, dir)
	f, err := os.Open(fullDir)
	if err != nil {
		fmt.Printf("error opening fullDir %v to count: %v\n", fullDir, err)
		return -1
	}
	list, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		fmt.Printf("error reading dirnames: %v\n", err)
		return -1
	}
	bs.existingFilenames = make(map[string]bool, len(list))
	for _, filename := range list {
		bs.existingFilenames[filename] = false
	}
	return len(list)
}

type worker struct {
	id          int
	stop        chan bool
	wg          *sync.WaitGroup
	baseDestDir string
	mu          *sync.Mutex
	client      *http.Client
}

func (w *worker) start(queue <-chan *mediaItemWrapper) {
	for {
		select {
		case miw := <-queue:
			if viper.GetBool("verbose") {
				fmt.Printf("Worker %v got %v of size %vw x %vh created at %v\n", w.id, miw.src.MimeType, miw.src.MediaMetadata.Width, miw.src.MediaMetadata.Height, miw.src.MediaMetadata.CreationTime)
			}

			err := w.ensureDestExists(miw)
			if err != nil {
				fmt.Printf("Error creating dest for %v, err: %v\n", miw.src.Filename, err)
				w.wg.Done()
				continue
			}
			if !w.fileExists(miw.destFilepath()) {
				data, err := w.fetchItem(miw)
				if err != nil {
					fmt.Printf("Error fetching %v, err: %v\n", miw.src.Filename, err)
					w.wg.Done()
					continue
				}
				if err = w.writeItem(miw, data); err != nil {
					fmt.Printf("Error writing %v, err: %v\n", miw.src.Filename, err)
					w.wg.Done()
					continue
				}
			} else {
				if viper.GetBool("verbose") {
					fmt.Printf("%v already exists\n", miw.shortDestFilepath())
				}
			}
			w.wg.Done()
		case <-w.stop:
			fmt.Printf("Worker %v received stop signal\n", w.id)
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
		fmt.Printf("Worker %v finished %v in %v\n", w.id, miw.shortDestFilepath(), time.Since(miw.startTime))
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

func wrap(mi *photoslibrary.MediaItem, baseDestDir string, destDirName string) *mediaItemWrapper {
	t, err := time.Parse(time.RFC3339, mi.MediaMetadata.CreationTime)
	if err != nil {
		fmt.Printf("Error parsing timestamp %v for id %v\n", mi.MediaMetadata.CreationTime, mi.Id)
	}
	return &mediaItemWrapper{
		src:          mi,
		baseDestDir:  baseDestDir,
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

func (miw *mediaItemWrapper) shortDestFilepath() string {
	return filepath.Join(miw.destDir(), miw.filename(true))
}

func (miw *mediaItemWrapper) filename(short bool) string {
	parts := strings.Split(miw.src.Filename, ".")
	var filename string
	if len(parts) == 2 {
		id := miw.src.Id
		if short && len(id) > 8 {
			id = fmt.Sprintf("...%v", id[len(id)-9:len(id)-1])
		}
		filename = fmt.Sprintf("%v-%v.%v", parts[0], id, parts[1])
	} else {
		filename = miw.src.Filename
	}
	return filename
}

func escapeAlbumTitle(t string) string {
	return specialChars.ReplaceAllString(t, "_")
}
