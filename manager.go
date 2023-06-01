package warplib

import "os"

type Manager struct {
	Items map[string]*Item
	f     *os.File
}

func InitManager() *Manager {
	return &Manager{}
}
