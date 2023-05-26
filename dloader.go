package warplib

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Downloader struct {
	Handlers                      Handlers
	fileName                      string
	contentLength                 ContentLength
	dlLoc                         string
	wg                            sync.WaitGroup
	chunk                         int
	url                           string
	client                        *http.Client
	numParts, currParts, maxParts int
	force                         bool
	ohmap                         VMap[int64, string]
	dlpath                        string
}

func NewDownloader(client *http.Client, url string, forceParts bool) (d *Downloader, err error) {
	d = &Downloader{
		wg:       sync.WaitGroup{},
		client:   client,
		url:      url,
		maxParts: 1,
		chunk:    int(DEF_CHUNK_SIZE),
		force:    forceParts,
	}
	err = d.fetchInfo()
	if err != nil {
		return
	}
	return d, d.setupDlPath()
}

func (d *Downloader) SetMaxParts(n int) {
	d.maxParts = n
}

func (d *Downloader) SetDownloadLocation(loc string) {
	d.dlLoc = strings.TrimSuffix(loc, "/")
}

func (d *Downloader) SetFileName(name string) {
	d.fileName = name
}

func (d *Downloader) GetParts() int {
	return d.numParts
}

func (d *Downloader) GetFileName() string {
	return d.fileName
}

func (d *Downloader) GetContentLength() ContentLength {
	return d.contentLength
}

func (d *Downloader) GetContentLengthAsInt() int64 {
	return d.GetContentLength().v()
}

func (d *Downloader) GetContentLengthAsString() string {
	return d.contentLength.String()
}

func (d *Downloader) Start() (err error) {
	d.ohmap.Make()
	partSize, rpartSize := d.getPartSize()
	for i := 0; i < d.numParts; i++ {
		i64 := int64(i)
		ioff := i64 * partSize
		foff := ioff + partSize - 1
		if i == d.numParts-1 {
			foff += rpartSize
		}
		d.wg.Add(1)
		go d.handlePart(ioff, foff, 4*MB)
	}
	d.wg.Wait()
	return d.compile()
}

func (d *Downloader) handlePart(ioff, foff, espeed int64) {
	d.currParts++
	part := d.spawnPart(ioff, foff, espeed)
	defer func() { d.currParts--; part.close(); d.wg.Done() }()
	slow, err := part.download(ioff, foff, false)
	if err != nil {
		d.Handlers.ErrorHandler(err)
		return
	}
	if !slow {
		return
	}
	ioff += part.read
	if d.currParts >= d.maxParts {
		_, err = part.download(ioff, foff, true)
		if err != nil {
			d.Handlers.ErrorHandler(err)
		}
		return
	}

	div := (foff - ioff) / 2

	d.wg.Add(1)
	go d.handlePart(ioff+div, foff, espeed/2)

	foff = ioff + div - 1
	d.Handlers.RespawnPartHandler(part.hash, part.read, ioff, foff)
	_, err = part.download(ioff, foff, true)
	if err != nil {
		d.Handlers.ErrorHandler(err)
	}
}

// func (d *Downloader) runPart(part *Part, ioff, foff, espeed int64) {
// 	slow, err := part.download(ioff, foff, false)
// 	if err != nil {
// 		d.Handlers.ErrorHandler(err)
// 		return
// 	}
// 	if !slow {
// 		return
// 	}
// 	ioff += part.read
// 	if d.currParts >= d.maxParts {
// 		_, err = part.download(ioff, foff, true)
// 		if err != nil {
// 			d.Handlers.ErrorHandler(err)
// 		}
// 		return
// 	}

// 	div := (foff - ioff) / 2

// 	d.wg.Add(1)
// 	go d.handlePart(ioff+div, foff, espeed/2)

// 	foff = ioff + div - 1

// 	d.Handlers.RespawnPartHandler(part.hash, part.read, ioff, foff)
// 	d.runPart(part, ioff, foff, espeed/2)
// }

func (d *Downloader) spawnPart(ioff, foff, espeed int64) (part *Part) {
	part = newPart(d.client, d.url, d.chunk, d.dlpath, d.Handlers.ProgressHandler)
	part.offset = ioff
	part.setEpeed(espeed)
	d.ohmap.Set(ioff, part.hash)
	d.Handlers.SpawnPartHandler(part.hash, ioff, foff)
	return
}

func (d *Downloader) setupDlPath() (err error) {
	tstamp := time.Now().UnixNano()
	dlpath := fmt.Sprintf(
		"%s/%s_%d/", ConfigDir, d.fileName, tstamp,
	)
	err = os.Mkdir(dlpath, os.ModePerm)
	if err != nil {
		return
	}
	d.dlpath = dlpath
	return
}

func (d *Downloader) fetchInfo() (err error) {
	resp, er := d.makeRequest(http.MethodGet)
	if er != nil {
		err = er
		return
	}
	defer resp.Body.Close()
	cd := resp.Header.Get("Content-Disposition")
	d.fileName = parseFileName(resp.Request, cd)
	err = d.setContentLength(resp.ContentLength)
	if err != nil {
		return
	}
	return d.prepareDownloader()
}

func (d *Downloader) setContentLength(cl int64) error {
	switch cl {
	case 0:
		return ErrContentLengthInvalid
	case 1:
		return errors.New("unknown size downloads not implemented yet")
	default:
		d.contentLength = ContentLength(cl)
		return nil
	}
}

func (d *Downloader) makeRequest(method string, hdrs ...Header) (*http.Response, error) {
	req, err := http.NewRequest(method, d.url, nil)
	if err != nil {
		return nil, err
	}
	header := req.Header
	setUserAgent(header)
	for _, hdr := range hdrs {
		hdr.Set(header)
	}
	return d.client.Do(req)
}

func (d *Downloader) prepareDownloader() (err error) {
	resp, er := d.makeRequest(
		http.MethodGet,
		Header{
			"Range", strings.Join(
				[]string{"bytes=1", strconv.Itoa(d.chunk)},
				"-",
			),
		},
	)
	if er != nil {
		err = er
		return
	}
	d.numParts = 1
	if !d.force && resp.Header.Get("Accept-Ranges") == "" {
		return
	}
	size := d.chunk
	if d.contentLength.v() < int64(size) {
		return
	}
	te, es := getSpeed(func() (err error) {
		buf := make([]byte, size)
		r, er := resp.Body.Read(buf)
		if er != nil {
			err = er
			return
		}
		if r < size {
			size = r
			return
		}
		return
	})
	if es != nil {
		err = es
		return
	}
	switch {
	case te > getDownloadTime(100*KB, int64(size)):
		// chunk is downloaded at a speed less than 100KB/s
		// very slow download
		d.numParts = 14
	case te > getDownloadTime(MB, int64(size)):
		// chunk is downloaded at a speed less than 1MB/s
		// slow download
		d.numParts = 8
	case te < getDownloadTime(10*MB, int64(size)):
		// chunk is downloaded at a speed more than 10MB/s
		// super fast download
		d.numParts = 2
	case te < getDownloadTime(5*MB, int64(size)):
		// chunk is downloaded at a speed more than 5MB/s
		// fast download
		d.numParts = 4
	}
	return
}

func (d *Downloader) getPartSize() (partSize, rpartSize int64) {
	cl := d.contentLength.v()
	partSize = cl / int64(d.numParts)
	rpartSize = cl % int64(d.numParts)
	return
}

func (d *Downloader) compile() (err error) {
	if d.dlLoc == "" {
		d.dlLoc = "."
	}
	svPath := strings.Join([]string{d.dlLoc, d.fileName}, "/")
	file, ef := os.Create(svPath)
	if ef != nil {
		err = ef
	}
	offsets := d.ohmap.Keys()
	if len(offsets) == 1 {
		hash := d.ohmap.GetUnsafe(offsets[0])
		fName := getFileName(d.dlpath, hash)
		err = os.Rename(fName, svPath)
		return
	}
	sortInt64s(offsets)
	for _, offset := range offsets {
		hash := d.ohmap.GetUnsafe(offset)
		fName := getFileName(
			d.dlpath,
			hash,
		)
		f, ef := os.Open(fName)
		if ef != nil {
			err = ef
			return
		}
		_, err = io.Copy(file, f)
		if err != nil {
			return
		}
		defer os.Remove(fName)
	}
	return
}
