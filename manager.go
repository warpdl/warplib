package warplib

import (
	"encoding/gob"
	"errors"
	"net/http"
	"os"
	"sync"
)

var __USERDATA_FILE_NAME = ConfigDir + "/userdata.warp"

type Manager struct {
	items ItemsMap
	f     *os.File
	mu    *sync.RWMutex
	wg    *sync.WaitGroup
	fmu   *sync.RWMutex
}

func InitManager() (m *Manager, err error) {
	m = &Manager{
		items: make(ItemsMap),
		mu:    new(sync.RWMutex),
		wg:    new(sync.WaitGroup),
		fmu:   new(sync.RWMutex),
	}
	m.f, err = os.OpenFile(
		__USERDATA_FILE_NAME,
		os.O_RDWR|os.O_CREATE,
		os.ModePerm,
	)
	if err != nil {
		m = nil
		return
	}
	_ = gob.NewDecoder(m.f).Decode(&m.items)
	m.populateMemPart()
	return
}

type AddDownloadOpts struct {
	IsHidden         bool
	IsChildren       bool
	Child            *Downloader
	AbsoluteLocation string
}

func (m *Manager) populateMemPart() {
	for _, item := range m.items {
		if item.memPart == nil {
			item.memPart = make(map[string]int64)
		}
		for ioff, part := range item.Parts {
			item.memPart[part.Hash] = ioff
		}
	}
}

func (m *Manager) AddDownload(d *Downloader, opts *AddDownloadOpts) (err error) {
	m.fmu.RLock()
	defer m.fmu.RUnlock()
	if opts == nil {
		opts = &AddDownloadOpts{}
	}
	cHash := ""
	if opts.Child != nil {
		cHash = opts.Child.hash
	}
	item, err := newItem(
		m.mu,
		d.fileName,
		d.url,
		d.dlLoc,
		d.hash,
		d.contentLength,
		&ItemOpts{
			AbsoluteLocation: opts.AbsoluteLocation,
			Child:            opts.IsChildren,
			Hide:             opts.IsHidden,
			ChildHash:        cHash,
		},
	)
	if err != nil {
		return err
	}
	m.wg.Add(1)
	m.UpdateItem(item)
	m.patchHandlers(d, item)
	return
}

func (m *Manager) patchHandlers(d *Downloader, item *Item) {
	oSPH := d.handlers.SpawnPartHandler
	d.handlers.SpawnPartHandler = func(hash string, ioff, foff int64) {
		item.addPart(hash, ioff, foff)
		m.UpdateItem(item)
		oSPH(hash, ioff, foff)
	}
	oRPH := d.handlers.RespawnPartHandler
	d.handlers.RespawnPartHandler = func(hash string, partIoff, ioffNew, foffNew int64) {
		item.addPart(hash, partIoff, foffNew)
		m.UpdateItem(item)
		oRPH(hash, partIoff, ioffNew, foffNew)
	}
	oPH := d.handlers.DownloadProgressHandler
	d.handlers.DownloadProgressHandler = func(hash string, nread int) {
		item.Downloaded += ContentLength(nread)
		m.UpdateItem(item)
		oPH(hash, nread)
	}
	oCCH := d.handlers.CompileCompleteHandler
	d.handlers.CompileCompleteHandler = func(hash string, tread int64) {
		off, part := item.getPart(hash)
		if part == nil {
			d.handlers.ErrorHandler(hash, errors.New("manager part item is nil"))
			return
		}
		part.Compiled = true
		item.savePart(off, part)
		oCCH(hash, tread)
	}
	oDCH := d.handlers.DownloadCompleteHandler
	d.handlers.DownloadCompleteHandler = func(hash string, tread int64) {
		if hash != MAIN_HASH {
			return
		}
		defer m.wg.Done()
		item.Parts = nil
		item.Downloaded = item.TotalSize
		m.UpdateItem(item)
		oDCH(hash, tread)
	}
}

func (m *Manager) encode(e any) (err error) {
	m.mu.Lock()
	m.f.Seek(0, 0)
	defer m.mu.Unlock()
	return gob.NewEncoder(m.f).Encode(e)
}

func (m *Manager) mapItem(item *Item) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[item.Hash] = item
}

func (m *Manager) UpdateItem(item *Item) {
	m.mapItem(item)
	m.encode(m.items)
}

func (m *Manager) GetItems() []*Item {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]*Item, len(m.items))
	var i int
	for _, item := range m.items {
		items[i] = item
		i++
	}
	return items
}

func (m *Manager) GetPublicItems() []*Item {
	var items = []*Item{}
	for _, item := range m.GetItems() {
		if item.Children {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (m *Manager) GetIncompleteItems() []*Item {
	var items = []*Item{}
	for _, item := range m.GetItems() {
		if item.TotalSize == item.Downloaded {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (m *Manager) GetCompletedItems() []*Item {
	var items = []*Item{}
	for _, item := range m.GetItems() {
		if item.TotalSize != item.Downloaded {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (m *Manager) GetItem(hash string) (item *Item) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item = m.items[hash]
	if item == nil {
		return
	}
	item.memPart = make(map[string]int64)
	item.mu = m.mu
	return
}

type ResumeDownloadOpts struct {
	ForceParts bool
	// MaxConnections sets the maximum number of parallel
	// network connections to be used for the downloading the file.
	MaxConnections int
	// MaxSegments sets the maximum number of file segments
	// to be created for the downloading the file.
	MaxSegments int
	Handlers    *Handlers
}

func (m *Manager) ResumeDownload(client *http.Client, hash string, opts *ResumeDownloadOpts) (item *Item, err error) {
	m.fmu.RLock()
	defer m.fmu.RUnlock()
	if opts == nil {
		opts = &ResumeDownloadOpts{}
	}
	item = m.GetItem(hash)
	if item == nil {
		err = ErrDownloadNotFound
		return
	}
	d, er := initDownloader(client, hash, item.Url, item.TotalSize, &DownloaderOpts{
		ForceParts:        opts.ForceParts,
		MaxConnections:    opts.MaxConnections,
		MaxSegments:       opts.MaxSegments,
		Handlers:          opts.Handlers,
		FileName:          item.Name,
		DownloadDirectory: item.DownloadLocation,
	})
	if er != nil {
		err = er
		return
	}
	m.wg.Add(1)
	m.patchHandlers(d, item)
	item.dAlloc = d
	return
}

func (m *Manager) Flush() error {
	m.wg.Wait()
	m.fmu.Lock()
	defer m.fmu.Unlock()
	m.items = make(ItemsMap)
	m.encode(m.items)
	return os.RemoveAll(DlDataDir)
}

func (m *Manager) Close() error {
	return m.f.Close()
}
