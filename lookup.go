package main

import (
	"fmt"
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

func (p *PodCache) Get(name string, namespace string) (*v1.Pod, bool) {
	key := fmt.Sprintf("%s.%s", namespace, name)
	p.RLock()
	entry, ok := p.cache[key]
	p.RUnlock()
	return entry, ok
}

func (p *PodCache) Delete(name string, namespace string) {
	key := fmt.Sprintf("%s.%s", namespace, name)
	p.Lock()
	defer p.Unlock()
	delete(p.cache, key)
}

func (p *PodCache) Set(name string, namespace string, pod *v1.Pod) {
	key := fmt.Sprintf("%s.%s", namespace, name)
	p.Lock()
	defer p.Unlock()
	p.cache[key] = pod
}
