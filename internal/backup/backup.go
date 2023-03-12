package backup

import (
	"context"
	"fmt"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Session struct {
	svc         *photoslibrary.Service
	queue       chan *photoslibrary.MediaItem
	wg          *sync.WaitGroup
	baseDestDir string
	workers     []*worker
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
		svc:         svc,
		queue:       make(chan *photoslibrary.MediaItem, 100),
		wg:          wg,
		baseDestDir: baseDestDir,
		workers:     workers,
	}, nil
}

func (bs *Session) Start(searchReq *photoslibrary.SearchMediaItemsRequest) {
	for _, w := range bs.workers {
		go w.start(bs.queue)
	}
	defer bs.Stop()

	err := bs.svc.MediaItems.Search(searchReq).
		Pages(context.Background(), func(resp *photoslibrary.SearchMediaItemsResponse) error {
			bs.wg.Add(len(resp.MediaItems))
			fmt.Printf("Adding %v items to queue\n", len(resp.MediaItems))

			for _, item := range resp.MediaItems {
				bs.queue <- item
			}
			return nil
		})
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
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

type worker struct {
	id          int
	stop        chan bool
	wg          *sync.WaitGroup
	baseDestDir string
	mu          *sync.Mutex
	client      *http.Client
}

func (w *worker) start(queue <-chan *photoslibrary.MediaItem) {
	for {
		select {
		case mi := <-queue:
			fmt.Printf("Worker %v got %v of size %vw x %vh created at %v\n", w.id, mi.MimeType, mi.MediaMetadata.Width, mi.MediaMetadata.Height, mi.MediaMetadata.CreationTime)
			miw := wrap(mi, w.baseDestDir)
			err := w.ensureDestExists(miw)
			if err != nil {
				fmt.Printf("Error creating dest for %v, err: %v\n", mi.Filename, err)
				w.wg.Done()
				continue
			}
			if !w.fileExists(miw.destFilepath()) {
				data, err := w.fetchItem(mi)
				if err != nil {
					fmt.Printf("Error fetching %v, err: %v\n", mi.Filename, err)
					w.wg.Done()
					continue
				}
				if err = w.writeItem(miw, data); err != nil {
					fmt.Printf("Error writing %v, err: %v\n", mi.Filename, err)
					w.wg.Done()
					continue
				}
			} else {
				fmt.Printf("%v already exists\n", miw.shortDestFilepath())
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

func (w *worker) fetchItem(mi *photoslibrary.MediaItem) ([]byte, error) {
	var url string
	switch {
	case mi.MediaMetadata.Video != nil:
		if mi.MediaMetadata.Video.Status != "READY" {
			return []byte{}, errors.Errorf("video %v is not yet processed", mi.Filename)
		}
		url = fmt.Sprintf("%v=dv", mi.BaseUrl)
	case mi.MediaMetadata.Photo != nil:
		url = fmt.Sprintf("%v=d", mi.BaseUrl)
	}

	if resp, err := w.client.Get(url); err != nil {
		return []byte{}, errors.Wrapf(err, "error fetching data for %v", mi.Filename)
	} else if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return []byte{}, errors.Errorf("Non 200 status returned for URL %v, body: %v", url, string(body))
	} else {
		return io.ReadAll(resp.Body)
	}
}

func (w *worker) writeItem(miw *mediaItemWrapper, data []byte) error {
	defer func() {
		fmt.Printf("Worker %v finished in %v\n", w.id, time.Now().Sub(miw.startTime))
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
}

func wrap(mi *photoslibrary.MediaItem, baseDestDir string) *mediaItemWrapper {
	t, err := time.Parse(time.RFC3339, mi.MediaMetadata.CreationTime)
	if err != nil {
		fmt.Printf("Error parsing timestamp %v for id %v", mi.MediaMetadata.CreationTime, mi.Id)
	}
	return &mediaItemWrapper{
		src:          mi,
		baseDestDir:  baseDestDir,
		creationTime: t,
		startTime:    time.Now(),
	}
}

func (miw *mediaItemWrapper) destDir() string {
	dir := "unknown"
	if !miw.creationTime.IsZero() {
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
