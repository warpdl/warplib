package warplib

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Part struct {
	// URL
	url string
	// size of a bytes chunk to be used for copying
	chunk int
	// unique hash for this part
	hash string
	// number of bytes downloaded
	read int64
	// progress handler
	pfunc ProgressHandlerFunc
	// dl compl handler
	ofunc DownloadCompleteHandlerFunc
	// http client
	client *http.Client
	// prename
	preName string
	// part file
	f *os.File
	// offset of part
	offset int64
	// expected speed
	espeed int64
	// logger
	l  *log.Logger
	wg *sync.WaitGroup
}

type partArgs struct {
	copyChunk int
	preName   string
	pHandler  ProgressHandlerFunc
	oHandler  DownloadCompleteHandlerFunc
	logger    *log.Logger
	offset    int64
}

func initPart(wg *sync.WaitGroup, client *http.Client, hash, url string, args partArgs) (*Part, error) {
	p := Part{
		url:     url,
		client:  client,
		chunk:   args.copyChunk,
		preName: args.preName,
		pfunc:   args.pHandler,
		ofunc:   args.oHandler,
		l:       args.logger,
		offset:  args.offset,
		hash:    hash,
		wg:      wg,
	}
	err := p.openPartFile()
	if err != nil {
		return nil, err
	}
	err = p.seek()
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func newPart(wg *sync.WaitGroup, client *http.Client, url string, args partArgs) (*Part, error) {
	p := Part{
		url:     url,
		client:  client,
		chunk:   args.copyChunk,
		preName: args.preName,
		pfunc:   args.pHandler,
		ofunc:   args.oHandler,
		l:       args.logger,
		offset:  args.offset,
		wg:      wg,
	}
	p.setHash()
	return &p, p.createPartFile()
}

func (p *Part) setEpeed(espeed int64) {
	p.espeed = espeed
}

func (p *Part) download(ioff, foff int64, force bool) (slow bool, err error) {
	req, er := http.NewRequest(http.MethodGet, p.url, nil)
	if er != nil {
		err = er
		return
	}
	header := req.Header
	setUserAgent(header)
	if foff != -1 {
		setRange(header, ioff, foff)
	} else {
		force = true
	}
	resp, er := p.client.Do(req)
	if er != nil {
		err = er
		return
	}
	defer resp.Body.Close()
	return p.copyBuffer(resp.Body, p.f, force)
}

func (p *Part) copyBuffer(src io.Reader, dst io.Writer, force bool) (slow bool, err error) {
	var (
		te  time.Duration
		buf = make([]byte, p.chunk)
	)
	var n int
	for {
		n++
		if !force && n%10 == 0 {
			te, err = getSpeed(func() error {
				return p.copyBufferChunk(src, dst, buf)
			})
			if err != nil {
				break
			}
			if te > getDownloadTime(p.espeed, int64(p.chunk)) {
				slow = true
				return
			}
			continue
		}
		err = p.copyBufferChunk(src, dst, buf)
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
		p.log("%s: part download complete", p.hash)
		p.wg.Add(1)
		go func() {
			p.ofunc(p.hash, p.read)
			p.wg.Done()
		}()
	}
	return
}

func (p *Part) copyBufferChunk(src io.Reader, dst io.Writer, buf []byte) (err error) {
	nr, er := src.Read(buf)
	if nr > 0 {
		nw, ew := dst.Write(buf[0:nr])
		if nw < 0 || nr < nw {
			nw = 0
			if ew == nil {
				ew = errors.New("invalid write results")
			}
		}
		p.read += int64(nw)
		p.wg.Add(1)
		go func() {
			p.pfunc(p.hash, nw)
			p.wg.Done()
		}()
		if ew != nil {
			err = ew
			return
		}
		if nr != nw {
			err = io.ErrShortWrite
			return
		}
	}
	err = er
	return
}

func setRange(header http.Header, ioff, foff int64) {
	str := func(i int64) string {
		return strconv.FormatInt(i, 10)
	}
	var b strings.Builder
	b.WriteString("bytes=")
	b.WriteString(str(ioff))
	b.WriteRune('-')
	if foff != 0 {
		b.WriteString(str(foff))
	}
	header.Set("Range", b.String())
}

func (p *Part) setHash() {
	t := make([]byte, 2)
	rand.Read(t)
	p.hash = hex.EncodeToString(t)
}

func (p *Part) createPartFile() (err error) {
	p.f, err = os.Create(p.getFileName())
	return
}

func (p *Part) openPartFile() (err error) {
	p.f, err = os.OpenFile(p.getFileName(), os.O_RDWR, 0666)
	return
}

func (p *Part) seek() (err error) {
	pReader := NewProxyReader(p.f, func(n int) {
		p.pfunc(p.hash, n)
	})
	n, err := io.Copy(io.Discard, pReader)
	if err != nil {
		return
	}
	p.read = n
	return
}

func (p *Part) getFileName() string {
	return getFileName(p.preName, p.hash)
}

func (p *Part) close() error {
	return p.f.Close()
}

func (p *Part) log(s string, a ...any) {
	p.l.Printf(s+"\n", a...)
}

func (p *Part) String() string {
	return p.hash
}
