package service

import (
	"sync"
)

type Registry interface {
	Register(serviceType any, service any) any
	Get(serviceType any) any
	UnRegister(serviceType any) //karing
	UnRegisterAll()             //karing
}

func NewRegistry() Registry {
	return &defaultRegistry{
		serviceTypes: make(map[any]any),
	}
}

type defaultRegistry struct {
	serviceTypes map[any]any
	access       sync.RWMutex
}

func (r *defaultRegistry) Register(serviceType any, service any) any {
	r.access.Lock()
	defer r.access.Unlock()
	oldService := r.serviceTypes[serviceType]
	r.serviceTypes[serviceType] = service
	return oldService
}

func (r *defaultRegistry) Get(serviceType any) any {
	r.access.RLock()
	defer r.access.RUnlock()
	return r.serviceTypes[serviceType]
}

func (r *defaultRegistry) UnRegister(serviceType any) { //karing
	r.access.Lock()
	defer r.access.Unlock()
	delete(r.serviceTypes, serviceType)
}

func (r *defaultRegistry) UnRegisterAll() { //karing
	r.access.Lock()
	defer r.access.Unlock()
	for k := range r.serviceTypes {
		delete(r.serviceTypes, k)
	}
}
