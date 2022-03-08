package rkex

import "sync"

// newAtomicMapFloat64 create thread safe map[string]float64
func newAtomicMapFloat64() *atomicMapFloat64 {
	return &atomicMapFloat64{
		lock: sync.Mutex{},
		m:    map[string]float64{},
	}
}

// atomicMapFloat64 thread safe
type atomicMapFloat64 struct {
	lock sync.Mutex
	m    map[string]float64
}

// Get get value
func (a *atomicMapFloat64) Get(key string) (float64, bool) {
	a.lock.Lock()
	defer a.lock.Unlock()

	v, ok := a.m[key]
	return v, ok
}

// Set K/V
func (a *atomicMapFloat64) Set(key string, val float64) {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.m[key] = val
}

// Delete key
func (a *atomicMapFloat64) Delete(key string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.m, key)
}

// Load insert full map
func (a *atomicMapFloat64) Load(src map[string]float64) {
	a.lock.Lock()
	defer a.lock.Unlock()

	for k, v := range src {
		a.m[k] = v
	}
}

// Copy copy to new map
func (a *atomicMapFloat64) Copy() map[string]float64 {
	a.lock.Lock()
	defer a.lock.Unlock()

	res := make(map[string]float64)

	for k, v := range a.m {
		res[k] = v
	}

	return res
}

// Empty is map empty?
func (a *atomicMapFloat64) Empty() bool {
	a.lock.Lock()
	defer a.lock.Unlock()

	return len(a.m) < 1
}

// Len length of map
func (a *atomicMapFloat64) Len() int {
	a.lock.Lock()
	defer a.lock.Unlock()

	return len(a.m)
}
