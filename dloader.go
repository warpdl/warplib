package warplib

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	// Max connections and number of curr connections
	maxConn, numConn int
	// Max spawnable parts and number of curr parts
	maxParts, numParts int
	// Initial number of parts to be spawned
	numBaseParts int
	// Setting force as 'true' will make downloader
	// split the file into segments even if it doesn't
	// have accept-ranges header.
	force bool
	// Handlers to be triggered while different events.
	handlers *Handlers
	// unique hash of this download
	hash string
	// headers to use for http requests
	headers Headers
	// total downloaded bytes
	nread  int64
	dlPath string
	wg     *sync.WaitGroup
	ohmap  VMap[int64, string]
	l      *log.Logger
	lw     io.WriteCloser
	f      *os.File
}

// Optional fields of downloader
type DownloaderOpts struct {
	ForceParts   bool
	NumBaseParts int
	// FileName is used to set name of to-be-downloaded
	// file explicitly.
	//
	// Note: Warplib sets the file name sent by server
	// if file name not set explicitly.
	FileName string
	// DownloadDirectory sets the download directory for
	// file to be downloaded.
	DownloadDirectory string
	// MaxConnections sets the maximum number of parallel
	// network connections to be used for the downloading the file.
	MaxConnections int
	// MaxSegments sets the maximum number of file segments
	// to be created for the downloading the file.
	MaxSegments int

	Headers Headers

	Handlers *Handlers

	SkipSetup bool
}

// NewDownloader creates a new downloader with provided arguments.
// Use downloader.Start() to download the file.
func NewDownloader(client *http.Client, url string, opts *DownloaderOpts) (d *Downloader, err error) {
	if opts == nil {
		opts = &DownloaderOpts{}
	}
	if opts.Handlers == nil {
		opts.Handlers = &Handlers{}
	}
	if opts.MaxConnections == 0 {
		opts.MaxConnections = DEF_MAX_CONNS
	}
	if opts.Headers == nil {
		opts.Headers = make(Headers, 0)
	}
	opts.Headers.InitOrUpdate(USER_AGENT_KEY, DEF_USER_AGENT)
	// loc := opts.DownloadDirectory
	// loc = strings.TrimSuffix(loc, "/")
	// if loc == "" {
	// 	loc = "."
	// }
	opts.DownloadDirectory, err = filepath.Abs(
		opts.DownloadDirectory,
	)
	if err != nil {
		return
	}
	d = &Downloader{
		wg:       &sync.WaitGroup{},
		client:   client,
		url:      url,
		maxConn:  opts.MaxConnections,
		chunk:    int(DEF_CHUNK_SIZE),
		force:    opts.ForceParts,
		handlers: opts.Handlers,
		fileName: opts.FileName,
		dlLoc:    opts.DownloadDirectory,
		maxParts: opts.MaxSegments,
		headers:  opts.Headers,
	}
	err = d.fetchInfo()
	if err != nil {
		return
	}
	if opts.SkipSetup {
		// Skip setting up dl path and stuff for a general download lookup.
		return
	}
	d.setHash()
	err = d.setupDlPath()
	if err != nil {
		return
	}
	err = d.setupLogger()
	if err != nil {
		return
	}
	d.handlers.setDefault(d.l)
	if opts.NumBaseParts != 0 {
		d.numBaseParts = opts.NumBaseParts
	}
	if d.maxParts != 0 && d.maxConn > d.maxParts {
		d.maxConn = d.maxParts
	}
	if d.numBaseParts > d.maxConn {
		d.numBaseParts = d.maxConn
	}
	if d.maxParts != 0 && d.numBaseParts > d.maxParts {
		d.numBaseParts = d.maxParts
	}
	return
}

func initDownloader(client *http.Client, hash, url string, cLength ContentLength, opts *DownloaderOpts) (d *Downloader, err error) {
	if opts == nil {
		opts = &DownloaderOpts{}
	}
	if opts.Handlers == nil {
		opts.Handlers = &Handlers{}
	}
	if opts.MaxConnections == 0 {
		opts.MaxConnections = DEF_MAX_CONNS
	}
	if opts.Headers == nil {
		opts.Headers = make(Headers, 0)
	}
	opts.Headers.InitOrUpdate(USER_AGENT_KEY, DEF_USER_AGENT)
	// loc := opts.DownloadDirectory
	// loc = strings.TrimSuffix(loc, "/")
	// if loc == "" {
	// 	loc = "."
	// }
	// opts.DownloadDirectory = loc
	opts.DownloadDirectory, err = filepath.Abs(
		opts.DownloadDirectory,
	)
	if err != nil {
		return
	}
	d = &Downloader{
		wg:            &sync.WaitGroup{},
		client:        client,
		url:           url,
		maxConn:       opts.MaxConnections,
		chunk:         int(DEF_CHUNK_SIZE),
		force:         opts.ForceParts,
		handlers:      opts.Handlers,
		fileName:      opts.FileName,
		dlLoc:         opts.DownloadDirectory,
		maxParts:      opts.MaxSegments,
		contentLength: cLength,
		hash:          hash,
		dlPath:        fmt.Sprintf("%s/%s/", DlDataDir, hash),
	}
	if !dirExists(d.dlPath) {
		err = errors.New("path to downloaded content doesn't exist")
		return
	}
	err = d.setupLogger()
	if err != nil {
		return
	}
	d.handlers.setDefault(d.l)
	if d.maxParts != 0 && d.maxConn > d.maxParts {
		d.maxConn = d.maxParts
	}
	return
}

// Start downloads the file and blocks current goroutine
// until the downloading is complete.
func (d *Downloader) Start() (err error) {
	defer d.lw.Close()
	err = d.openFile()
	if err != nil {
		return
	}
	defer func() {
		d.f.Close()
		// err = os.Rename(d.fName, d.GetSavePath())
	}()
	d.Log("Starting download...")
	d.ohmap.Make()
	partSize, rpartSize := d.getPartSize()
	for i := 0; i < d.numBaseParts; i++ {
		ioff := int64(i) * partSize
		foff := ioff + partSize - 1
		if i == d.numBaseParts-1 {
			foff += rpartSize
		}
		d.wg.Add(1)
		go d.newPartDownload(ioff, foff, 4*MB)
	}
	d.wg.Wait()
	if d.contentLength.v() != d.nread {
		d.Log("Download failed", "Expected bytes:", d.contentLength, "Found bytes:", d.nread)
		return
	}
	d.handlers.DownloadCompleteHandler(MAIN_HASH, d.contentLength.v())
	d.Log("All segments downloaded!")
	return
}

// TODO: fix concurrent write and iteration if any.

// map[InitialOffset(int64)]ItemPart
func (d *Downloader) Resume(parts map[int64]*ItemPart) (err error) {
	defer d.lw.Close()
	if len(parts) == 0 {
		return errors.New("download is already complete")
	}
	err = d.openFile()
	if err != nil {
		return
	}
	defer func() {
		d.f.Close()
		// err = os.Rename(d.fName, d.GetSavePath())
	}()
	d.Log("Resuming download...")
	d.ohmap.Make()
	espeed := 4 * MB / int64(len(parts))
	for ioff, ip := range parts {
		if ip.Compiled {
			d.handlers.CompileSkippedHandler(ip.Hash, ip.FinalOffset-ioff)
			continue
		}
		d.wg.Add(1)
		go d.resumePartDownload(ip.Hash, ioff, ip.FinalOffset, espeed)
	}
	d.wg.Wait()
	if d.contentLength.v() != d.nread {
		d.Log("Download failed", "Expected bytes:", d.contentLength, "Found bytes:", d.nread)
		return
	}
	d.handlers.DownloadCompleteHandler(MAIN_HASH, d.contentLength.v())
	d.Log("All segments downloaded!")
	return
}

func (d *Downloader) openFile() (err error) {
	// d.fName = d.dlPath + "warp.dl"
	d.f, err = os.OpenFile(d.GetSavePath(),
		os.O_RDWR|os.O_CREATE,
		0666,
	)
	return
}

func (d *Downloader) spawnPart(ioff, foff int64) (part *Part, err error) {
	part, err = newPart(
		d.wg,
		d.client,
		d.url,
		partArgs{
			d.chunk,
			d.dlPath,
			d.handlers.ResumeProgressHandler,
			d.handlers.DownloadProgressHandler,
			d.handlers.DownloadCompleteHandler,
			d.handlers.CompileProgressHandler,
			d.l,
			ioff,
			d.f,
		},
	)
	if err != nil {
		return
	}
	// part.offset = ioff
	d.ohmap.Set(ioff, part.hash)
	d.numParts++
	d.Log("%s: Created new part", part.hash)
	d.handlers.SpawnPartHandler(part.hash, ioff, foff)
	return
}

func (d *Downloader) initPart(hash string, ioff, foff int64) (part *Part, err error) {
	part, err = initPart(
		d.wg,
		d.client,
		hash,
		d.url,
		partArgs{
			d.chunk,
			d.dlPath,
			d.handlers.ResumeProgressHandler,
			d.handlers.DownloadProgressHandler,
			d.handlers.DownloadCompleteHandler,
			d.handlers.CompileProgressHandler,
			d.l,
			ioff,
			d.f,
		},
	)
	if err != nil {
		return
	}
	d.ohmap.Set(ioff, hash)
	d.numParts++
	d.Log("%s: Resumed part", hash)
	d.handlers.SpawnPartHandler(hash, ioff, foff)
	return
}

func (d *Downloader) resumePartDownload(hash string, ioff, foff, espeed int64) {
	d.numConn++
	part, err := d.initPart(hash, ioff, foff)
	if err != nil {
		d.Log("%s: init: %w", hash, err)
		return
	}
	defer func() { d.numConn--; d.wg.Done() }()
	poff := part.offset + part.read
	if poff >= foff {
		d.Log("%s: part offset (%d) greater than final offset (%d)", hash, poff, foff)
		return
	}
	err = d.runPart(part, poff, foff, espeed, false)
	d.nread += part.read
	if err != nil {
		return
	}
	d.handlers.CompileStartHandler(part.hash)
	defer d.handlers.CompileCompleteHandler(part.hash, part.read)

	d.Log("%s: compiling part", hash)

	var read, written int64
	read, written, err = part.compile()

	// close part file
	part.close()

	if err != nil {
		d.Log("%s: compile: %w", hash, err)
		return
	}
	d.Log("%s: compilation complete: read %d bytes and wrote %d bytes", hash, read, written)

	fName := getFileName(
		d.dlPath,
		hash,
	)
	err = os.Remove(fName)
	if err == nil {
		return
	}
	d.Log("%s: remove: %w", hash, err)
}

func (d *Downloader) newPartDownload(ioff, foff, espeed int64) {
	d.numConn++
	part, err := d.spawnPart(ioff, foff)
	if err != nil {
		d.Log("failed to spawn new part: %w", err)
		return
	}
	hash := part.hash

	defer func() { d.numConn--; d.wg.Done() }()
	err = d.runPart(part, ioff, foff, espeed, false)
	d.nread += part.read
	if err != nil {
		return
	}

	d.handlers.CompileStartHandler(part.hash)
	defer d.handlers.CompileCompleteHandler(part.hash, part.read)

	d.Log("%s: compiling part", hash)

	var read, written int64
	read, written, err = part.compile()

	// close part file
	part.close()

	if err != nil {
		d.Log("%s: compile: %w", hash, err)
		return
	}
	d.Log("%s: compilation complete: read %d bytes and wrote %d bytes", hash, read, written)

	fName := getFileName(
		d.dlPath,
		hash,
	)
	err = os.Remove(fName)
	if err == nil {
		return
	}
	d.Log("%s: remove: %w", hash, err)
}

// runPart downloads the content starting from ioff till foff bytes
// offset. espeed stands for expected download speed which, slower
// download speed than this espeed will result in spawning a new part
// if a slot is available for it and maximum parts limit is not reached.
func (d *Downloader) runPart(part *Part, ioff, foff, espeed int64, repeated bool) error {
	hash := part.hash
	// set espeed each time the runPart function is called to update
	// the older espeed present in respawned parts.
	part.setEpeed(espeed)
	if !repeated {
		d.Log("%s: Set part espeed to %s", hash, ContentLength(espeed))
		d.Log("%s: Started downloading part", hash)
	}

	// start downloading the content in provided
	// offset range until part becomes slower than
	// expected speed.
	slow, err := part.download(d.headers, ioff, foff, false)
	if err != nil {
		d.handlers.ErrorHandler(hash, err)
		return err
	}
	if !slow {
		return nil
	}
	d.Log("%s: Detected part as running slow", hash)

	// add read bytes to part offset to determine
	// starting offset for a resplit download.
	poff := part.offset + part.read

	if d.maxParts != 0 && d.numParts >= d.maxParts {
		// Max part limit has been reached and hence
		// don't spawn new parts and forcefully download
		// rest of the content in slow part.
		d.Log("%s: Max part limit reached, continuing slow part...", hash)
		_, err = part.download(d.headers, poff, foff, true)
		if err != nil {
			d.handlers.ErrorHandler(hash, err)
			return err
		}
	}
	if d.maxConn != 0 && d.numConn >= d.maxConn {
		// It waits until a connection is
		// freed and spawns a new part once
		// a slot is available.
		// Part is continued if the speed gets
		// better before it gets a new slot.
		return d.runPart(part, poff, foff, espeed, true)
	}

	// divide the pending bytes of current slow
	// part among the current part and a newly
	// spawned part.
	div := (foff - poff) / 2

	// spawn a new part and add its goroutine to
	// waitgroup, new part will download the last
	// 2nd half of pending bytes.
	d.wg.Add(1)
	go d.newPartDownload(poff+div, foff, espeed/2)

	// current part will download the first half
	// of pending bytes.
	foff = poff + div - 1

	d.Log("%s: part respawned", hash)
	d.handlers.RespawnPartHandler(hash, part.offset, poff, foff)
	return d.runPart(part, poff, foff, espeed/2, false)
}

func (d *Downloader) GetFileName() string {
	return d.fileName
}

func (d *Downloader) GetDownloadDirectory() string {
	return d.dlLoc
}

func (d *Downloader) GetSavePath() (svPath string) {
	svPath = GetPath(d.dlLoc, d.fileName)
	return
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
	return d.numConn
}

// Log adds the provided string to download's log file.
// It can't be used once download is complete.
func (d *Downloader) Log(s string, a ...any) {
	wlog(d.l, s, a...)
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
	if d.fileName != "" {
		return
	}
	cd := h.Get("Content-Disposition")
	d.fileName = parseFileName(r, cd)
}

func (d *Downloader) setHash() {
	buf := make([]byte, 4)
	rand.Read(buf)
	d.hash = hex.EncodeToString(buf)
}

func (d *Downloader) setupDlPath() (err error) {
	dlpath := fmt.Sprintf(
		"%s/%s/", DlDataDir, d.hash,
	)
	err = os.Mkdir(dlpath, os.ModePerm)
	if err != nil {
		return
	}
	d.dlPath = dlpath
	return
}

func (d *Downloader) setupLogger() (err error) {
	d.lw, err = os.OpenFile(
		d.dlPath+"logs.txt",
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		return
	}
	d.l = log.New(d.lw, "", log.LstdFlags)
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
	for _, hdr := range hdrs {
		hdr.Set(header)
	}
	d.headers.Set(header)
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
