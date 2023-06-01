package warplib

import (
	"encoding/gob"
	"os"
	"sync"
)

type Manager struct {
	items ItemsMap
	f     *os.File
	mu    *sync.RWMutex
}

func (m *Manager) AddDownload(d *Downloader) {
	item := newItem(
		d.fileName,
		d.url,
		d.dlLoc,
		d.hash,
		d.contentLength,
		m.mu,
	)
	m.UpdateItem(item)
	oSPH := d.handlers.SpawnPartHandler
	d.handlers.SpawnPartHandler = func(hash string, ioff, foff int64) {
		item.addPart(ioff, hash)
		defer m.UpdateItem(item)
		oSPH(hash, ioff, foff)
	}
	oPH := d.handlers.ProgressHandler
	d.handlers.ProgressHandler = func(hash string, nread int) {
		item.Downloaded += ContentLength(nread)
		defer m.UpdateItem(item)
		oPH(hash, nread)
	}
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

func (m *Manager) encode(e any) (err error) {
	m.mu.Lock()
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
