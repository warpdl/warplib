package warplib

import (
	"encoding/gob"
	"net/http"
	"os"
	"sync"
)

type Manager struct {
	items ItemsMap
	f     *os.File
	mu    *sync.RWMutex
}

func InitManager() (m *Manager, err error) {
	m = &Manager{
		items: make(ItemsMap),
		mu:    new(sync.RWMutex),
	}
	fn := ConfigDir + "/userdata.warp"
	m.f, err = os.OpenFile(fn, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		m = nil
		return
	}
	_ = gob.NewDecoder(m.f).Decode(&m.items)
	return
}

type AddDownloadOpts struct {
	IsHidden         bool
	IsChildren       bool
	Child            *Downloader
	AbsoluteLocation string
}

func (m *Manager) AddDownload(d *Downloader, opts *AddDownloadOpts) (err error) {
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
	d.handlers.CompileCompleteHandler = func() {
		item.Parts = nil
		item.Downloaded = item.TotalSize
		m.UpdateItem(item)
		oCCH()
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
	m.patchHandlers(d, item)
	item.dAlloc = d
	return
}

func (m *Manager) Close() error {
	return m.f.Close()
}
