package warplib

import (
	"sync"
)

type VMap[kT comparable, vT any] struct {
	kv map[kT]vT
	mu sync.RWMutex
}

func (vm *VMap[kT, vT]) Make() {
	vm.kv = make(map[kT]vT)
}

func (vm *VMap[kT, vT]) Set(key kT, val vT) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.kv[key] = val
}

func (vm *VMap[kT, vT]) GetUnsafe(key kT) (val vT) {
	val = vm.kv[key]
	return
}

func (vm *VMap[kT, vT]) Get(key kT) (val vT) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	val = vm.GetUnsafe(key)
	return
}

func (vm *VMap[kT, vT]) Keys() (keys []kT) {
	keys = make([]kT, len(vm.kv))

	vm.mu.Lock()
	defer vm.mu.Unlock()

	var i int
	for key := range vm.kv {
		keys[i] = key
		i++
	}
	return
}
