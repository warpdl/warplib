package warplib

import (
	"strings"
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
	AbsoluteLocation string
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
	Hide, Child      bool
	ChildHash        string
	AbsoluteLocation string
}

func newItem(mu *sync.RWMutex, name, url, dlloc, hash string, totalSize ContentLength, opts *ItemOpts) *Item {
	if opts == nil {
		opts = &ItemOpts{}
	}
	opts.AbsoluteLocation = strings.TrimSuffix(
		opts.AbsoluteLocation, "/",
	)
	return &Item{
		Hash:             hash,
		Name:             name,
		Url:              url,
		DateAdded:        time.Now(),
		TotalSize:        totalSize,
		DownloadLocation: dlloc,
		AbsoluteLocation: opts.AbsoluteLocation,
		ChildHash:        opts.ChildHash,
		Hidden:           opts.Hide,
		Children:         opts.Child,
		Parts:            make(map[int64]ItemPart),
		mu:               mu,
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

func (i *Item) GetAbsolutePath() (aPath string) {
	aPath = GetPath(i.AbsoluteLocation, i.Name)
	return
}

func (i *Item) Resume() error {
	return i.dAlloc.Resume(i.Parts)
}
