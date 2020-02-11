package main

import (
	"sync"

	v1 "k8s.io/api/core/v1"
)

type PodCache struct {
	sync.RWMutex
	cache map[string]*v1.Pod
}

// NewPodCache creates a new PodCache
func NewPodCache() *PodCache {
	podCache := new(PodCache)
	podCache.cache = make(map[string]*v1.Pod)
	return podCache
}

func (p *PodCache) Get(name string) (*v1.Pod, bool) {
	p.RLock()
	entry, ok := p.cache[name]
	p.RUnlock()
	return entry, ok
}

func (p *PodCache) Delete(name string) {
	p.Lock()
	defer p.Unlock()
	delete(p.cache, name)
}

func (p *PodCache) Set(name string, pod *v1.Pod) {
	p.Lock()
	defer p.Unlock()
	p.cache[name] = pod
}
