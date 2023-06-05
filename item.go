package warplib

import (
	"sync"
	"time"
)

type Item struct {
	Hash             string
	Name             string
	Url              string
	DateAdded        time.Time
	TotalSize        ContentLength
	Downloaded       ContentLength
	DownloadLocation string
	ChildHash        string
	Hidden           bool
	Children         bool
	Parts            map[int64]ItemPart
	mu               *sync.RWMutex
	dAlloc           *Downloader
}

type ItemPart struct {
	Hash        string
	FinalOffset int64
}

type ItemsMap map[string]*Item

type ItemOpts struct {
	Hide, Child bool
	ChildHash   string
}

func newItem(mu *sync.RWMutex, name, url, dlloc, hash string, totalSize ContentLength, opts *ItemOpts) *Item {
	if opts == nil {
		opts = &ItemOpts{}
	}
	return &Item{
		Name:      name,
		Url:       url,
		Hash:      hash,
		TotalSize: totalSize,
		Parts:     make(map[int64]ItemPart),
		DateAdded: time.Now(),
		Hidden:    opts.Hide,
		Children:  opts.Child,
		ChildHash: opts.ChildHash,
		mu:        mu,
	}
}

func (i *Item) addPart(hash string, ioff, foff int64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Parts[ioff] = ItemPart{
		Hash:        hash,
		FinalOffset: foff,
	}
}

func (i *Item) GetPercentage() int64 {
	p := (i.Downloaded * 100) / i.TotalSize
	return p.v()
}

func (i *Item) GetSavePath() (svPath string) {
	svPath = GetPath(i.DownloadLocation, i.Name)
	return
}

func (i *Item) Resume() error {
	return i.dAlloc.Resume(i.Parts)
}
