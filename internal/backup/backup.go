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
	"sync"
	"time"
)

type Session struct {
	svc     *photoslibrary.Service
	queue   chan *photoslibrary.MediaItem
	wg      *sync.WaitGroup
	dest    string
	workers []*worker
}

func NewSession(client *http.Client, dest string, workerCount int) (*Session, error) {
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
			dest:   dest,
			mu:     mu,
			client: client,
		}
	}

	return &Session{
		svc:     svc,
		queue:   make(chan *photoslibrary.MediaItem, 100),
		wg:      wg,
		dest:    dest,
		workers: workers,
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
			fmt.Printf("Adding %v items to queue", len(resp.MediaItems))

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
	id     int
	stop   chan bool
	wg     *sync.WaitGroup
	dest   string
	mu     *sync.Mutex
	client *http.Client
}

func (w *worker) start(queue <-chan *photoslibrary.MediaItem) {
	for {
		select {
		case mi := <-queue:
			fmt.Printf("Worker %v got item ID %v created at %v\n", w.id, mi.Id, mi.MediaMetadata.CreationTime)
			destFilepath, err := w.ensureDestExists(mi.MediaMetadata.CreationTime)
			if err != nil {
				fmt.Printf("Error creating dest for %v, err: %v\n", mi.Filename, err)
				w.wg.Done()
				continue
			}
			data, err := w.fetchItem(mi)
			if err != nil {
				fmt.Printf("Error fetching %v, err: %v\n", mi.Filename, err)
				w.wg.Done()
				continue
			}
			if err = w.writeItem(mi, data, destFilepath); err != nil {
				fmt.Printf("Error writing %v, err: %v\n", mi.Filename, err)
				w.wg.Done()
				continue
			}
			w.wg.Done()
		case <-w.stop:
			fmt.Printf("Worker %v received stop signal\n", w.id)
			w.wg.Done()
			return
		}
	}
}

func (w *worker) ensureDestExists(creationTimeStr string) (string, error) {
	dirs := "unknown"
	if creationTimeStr != "" {
		t, err := time.Parse(time.RFC3339, creationTimeStr)
		if err != nil {
			fmt.Printf("Error parsing timestamp %v", creationTimeStr)
		}
		dirs = t.Format("2006/01/02")
	}
	destDirs := filepath.Join(w.dest, dirs)
	w.mu.Lock()
	err := os.MkdirAll(destDirs, 0755)
	w.mu.Unlock()
	if err != nil {
		return "", errors.Wrapf(err, "error creating dest dir %v", destDirs)
	}
	return destDirs, nil
}

func (w *worker) fetchItem(mi *photoslibrary.MediaItem) ([]byte, error) {
	url := fmt.Sprintf("%v=h%v-w%v", mi.BaseUrl, mi.MediaMetadata.Height, mi.MediaMetadata.Width)
	if resp, err := w.client.Get(url); err != nil {
		return []byte{}, nil
	} else if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return []byte{}, errors.Errorf("Non 200 status returned for URL %v, body: %v", url, string(body))
	} else {
		return io.ReadAll(resp.Body)
	}
}

func (w *worker) writeItem(mi *photoslibrary.MediaItem, data []byte, dest string) error {
	filename := fmt.Sprintf("%v/%v", dest, mi.Filename)

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return errors.Wrapf(err, "writing item %v", mi.Id)
	}
	return nil
}
