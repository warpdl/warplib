package warplib

import (
	"strings"
	"sync"
	"time"
)

type Item struct {
	Name             string
	Url              string
	DateAdded        time.Time
	TotalSize        ContentLength
	Downloaded       ContentLength
	DownloadLocation string
	Hash             string
	Parts            map[int64]string
	mu               *sync.RWMutex
}

type ItemsMap map[string]*Item

func newItem(name, url, dlloc, hash string, totalSize ContentLength, mu *sync.RWMutex) *Item {
	return &Item{
		Name:      name,
		Url:       url,
		Hash:      hash,
		TotalSize: totalSize,
		Parts:     make(map[int64]string),
		DateAdded: time.Now(),
		mu:        mu,
	}
}

func (i *Item) addPart(ioff int64, hash string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Parts[ioff] = hash
}

func (i *Item) GetPercentage() int64 {
	p := (i.Downloaded / i.TotalSize) * 100
	return p.v()
}

func (i *Item) GetSaveLocation() string {
	svPath := strings.Join(
		[]string{
			i.DownloadLocation, i.Name,
		},
		"/",
	)
	return svPath
}
