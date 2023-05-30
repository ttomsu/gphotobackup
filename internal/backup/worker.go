package backup

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

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
