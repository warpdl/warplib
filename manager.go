package warplib

import (
	"encoding/gob"
	"io"
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

	err = gob.NewDecoder(m.f).Decode(&m.items)
	if err == io.EOF {
		err = nil
	}
	return
}

type AddDownloadOpts struct {
	IsHidden   bool
	IsChildren bool
	Child      *Downloader
}

func (m *Manager) AddDownload(d *Downloader, opts *AddDownloadOpts) {
	if opts == nil {
		opts = &AddDownloadOpts{}
	}
	cHash := ""
	if opts.Child != nil {
		cHash = opts.Child.hash
	}
	item := newItem(
		m.mu,
		d.fileName,
		d.url,
		d.dlLoc,
		d.hash,
		d.contentLength,
		&ItemOpts{
			Child:     opts.IsChildren,
			Hide:      opts.IsHidden,
			ChildHash: cHash,
		},
	)
	m.UpdateItem(item)
	oSPH := d.handlers.SpawnPartHandler
	d.handlers.SpawnPartHandler = func(hash string, ioff, foff int64) {
		item.addPart(ioff, hash)
		m.UpdateItem(item)
		oSPH(hash, ioff, foff)
	}
	oPH := d.handlers.ProgressHandler
	d.handlers.ProgressHandler = func(hash string, nread int) {
		item.Downloaded += ContentLength(nread)
		m.UpdateItem(item)
		oPH(hash, nread)
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

func (m *Manager) GetItems() ItemsMap {
	return m.items
}

func (m *Manager) GetItem(hash string) (item *Item) {
	m.mu.RLock()
	item = m.items[hash]
	m.mu.RUnlock()
	return
}

func (m *Manager) Close() error {
	return m.f.Close()
}
