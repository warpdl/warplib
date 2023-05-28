package warplib

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Downloader struct {
	// Http client to be used to for the whole process
	client *http.Client
	// Url of the file to be downloaded
	url string
	// File name to be used while saving it
	fileName string
	// Size of file, wrapped inside ContentLength
	contentLength ContentLength
	// Download location (directory) of the file.
	dlLoc string
	// Size of 1 chunk of bytes to download during
	// a single copy cycle
	chunk int
	// max connections and number of curr connections
	maxConn, numConn int
	// max spawnable parts and number of curr parts
	maxParts, numParts int
	// initial number of parts to be spawned
	numBaseParts int
	// setting force as 'true' will make downloader
	// split the file into segments even if it doesn't
	// have accept-ranges header.
	force bool
	// Handlers to be triggered while different events.
	Handlers *Handlers
	dlPath   string
	wg       sync.WaitGroup
	ohmap    VMap[int64, string]
}

// NewDownloader creates a new downloader with provided arguments.
// Use downloader.Start() to download the file.
func NewDownloader(client *http.Client, url string, forceParts bool) (d *Downloader, err error) {
	d = &Downloader{
		wg:      sync.WaitGroup{},
		client:  client,
		url:     url,
		maxConn: 1,
		chunk:   int(DEF_CHUNK_SIZE),
		force:   forceParts,
	}
	err = d.fetchInfo()
	if err != nil {
		return
	}
	return d, d.setupDlPath()
}

// Start downloads the file and blocks current goroutine
// until the downloading is complete.
func (d *Downloader) Start() (err error) {
	d.ohmap.Make()
	partSize, rpartSize := d.getPartSize()
	for i := 0; i < d.numBaseParts; i++ {
		ioff := int64(i) * partSize
		foff := ioff + partSize - 1
		if i == d.numBaseParts-1 {
			foff += rpartSize
		}
		d.wg.Add(1)
		go d.handlePart(ioff, foff, 4*MB)
	}
	d.wg.Wait()
	return d.compile()
}

func (d *Downloader) handlePart(ioff, foff, espeed int64) {
	d.numConn++
	part := d.spawnPart(ioff, foff, espeed)
	defer func() { d.numConn--; part.close(); d.wg.Done() }()
	d.runPart(part, ioff, foff, espeed)
}

func (d *Downloader) spawnPart(ioff, foff, espeed int64) (part *Part) {
	part = newPart(d.client, d.url, d.chunk, d.dlPath, d.Handlers.ProgressHandler, d.Handlers.OnCompleteHandler)
	part.offset = ioff
	d.ohmap.Set(ioff, part.hash)
	d.numParts++
	d.Handlers.SpawnPartHandler(part.hash, ioff, foff)
	return
}

// runPart downloads the content starting from ioff till foff bytes
// offset. espeed stands for expected download speed which, slower
// download speed than this espeed will result in spawning a new part
// if a slot is available for it and maximum parts limit is not reached.
func (d *Downloader) runPart(part *Part, ioff, foff, espeed int64) {
	// set espeed each time the runPart function is called to update
	// the older espeed present in respawned parts.
	part.setEpeed(espeed)

	// start downloading the content in provided
	// offset range until part becomes slower than
	// expected speed.
	slow, err := part.download(ioff, foff, false)
	if err != nil {
		d.Handlers.ErrorHandler(err)
		return
	}
	if !slow {
		return
	}

	// add read bytes to part offset to determine
	// starting offset for a resplit download.
	poff := part.offset + part.read

	if d.maxParts != 0 && d.numParts >= d.maxParts {
		// Max part limit has been reached and hence
		// don't spawn new parts and forcefully download
		// rest of the content in slow part.
		_, err := part.download(poff, foff, true)
		if err != nil {
			d.Handlers.ErrorHandler(err)
			return
		}
	}
	if d.maxConn != 0 && d.numConn >= d.maxConn {
		// It waits until a connection is
		// freed and spawns a new part once
		// a slot is available.
		// Part is continued if the speed gets
		// better before it gets a new slot.
		d.runPart(part, poff, foff, espeed)
		return
	}

	// divide the pending bytes of current slow
	// part among the current part and a newly
	// spawned part.
	div := (foff - poff) / 2

	// spawn a new part and add its goroutine to
	// waitgroup, new part will download the last
	// 2nd half of pending bytes.
	d.wg.Add(1)
	go d.handlePart(poff+div, foff, espeed/2)

	// current part will download the first half
	// of pending bytes.
	foff = poff + div - 1
	d.Handlers.RespawnPartHandler(part.hash, poff, foff)
	d.runPart(part, poff, foff, espeed/2)
}

// SetMaxConnections sets the maximum number of parallel
// network connections to be used for the downloading the file.
func (d *Downloader) SetMaxConnections(n int) {
	if d.numBaseParts > n {
		d.numBaseParts = n
	}
	d.maxConn = n
}

// SetMaxParts sets the maximum number of file segments
// to be created for the downloading the file.
func (d *Downloader) SetMaxParts(n int) {
	if d.numBaseParts > n {
		d.numBaseParts = n
	}
	d.maxParts = n
}

// SetDownloadLocation sets the download directory for
// file to be downloaded.
func (d *Downloader) SetDownloadLocation(loc string) {
	d.dlLoc = strings.TrimSuffix(loc, "/")
}

// SetFileName is used to set name of to-be-downloaded
// file explicitly.
//
// Note: Warplib sets the file name sent by server
// if file name not set explicitly.
func (d *Downloader) SetFileName(name string) {
	if name == "" {
		return
	}
	d.fileName = name
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

// NumConnections returns the number of connections
// running currently.
func (d *Downloader) NumConnections() int {
	return d.numBaseParts
}

func (d *Downloader) getPartSize() (partSize, rpartSize int64) {
	switch cl := d.contentLength.v(); cl {
	case -1, 0:
		partSize = -1
	default:
		partSize = cl / int64(d.numBaseParts)
		rpartSize = cl % int64(d.numBaseParts)
	}
	return
}

func (d *Downloader) setContentLength(cl int64) error {
	switch cl {
	case 0:
		return ErrContentLengthInvalid
	case -1:
		return ErrContentLengthNotImplemented
	default:
		d.contentLength = ContentLength(cl)
		return nil
	}
}

func (d *Downloader) setFileName(r *http.Request, h *http.Header) {
	cd := h.Get("Content-Disposition")
	d.fileName = parseFileName(r, cd)
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
	d.dlPath = dlpath
	return
}

func (d *Downloader) checkContentType(h *http.Header) (err error) {
	ct := h.Get("Content-Type")
	if ct == "" {
		return
	}
	switch ct, _, _ = mime.ParseMediaType(ct); ct {
	case "text/html", "text/css":
		err = ErrNotSupported
	}
	return
}

func (d *Downloader) fetchInfo() (err error) {
	resp, er := d.makeRequest(http.MethodGet)
	if er != nil {
		err = er
		return
	}
	defer resp.Body.Close()
	h := resp.Header
	err = d.checkContentType(&h)
	if err != nil {
		return
	}
	err = d.setContentLength(resp.ContentLength)
	if err != nil {
		return
	}
	d.setFileName(resp.Request, &h)
	return d.prepareDownloader()
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
	d.numBaseParts = 1
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
		d.numBaseParts = 14
	case te > getDownloadTime(MB, int64(size)):
		// chunk is downloaded at a speed less than 1MB/s
		// slow download
		d.numBaseParts = 12
	case te < getDownloadTime(10*MB, int64(size)):
		// chunk is downloaded at a speed more than 10MB/s
		// super fast download
		d.numBaseParts = 8
	case te < getDownloadTime(5*MB, int64(size)):
		// chunk is downloaded at a speed more than 5MB/s
		// fast download
		d.numBaseParts = 10
	}
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
		fName := getFileName(d.dlPath, hash)
		err = os.Rename(fName, svPath)
		return
	}
	sortInt64s(offsets)
	for _, offset := range offsets {
		hash := d.ohmap.GetUnsafe(offset)
		fName := getFileName(
			d.dlPath,
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
