package warplib

import "time"

type Item struct {
	Name             string
	Url              string
	DateAdded        time.Time
	TotalSize        ContentLength
	Downloaded       ContentLength
	DownloadLocation string
	Hash             string
}
