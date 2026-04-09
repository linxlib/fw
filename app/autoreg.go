package app

import "sync"

type AutoRegistrar func(*Engine) error

var (
	autoRegistrarMu sync.RWMutex
	autoRegistrars  []AutoRegistrar
)

func RegisterAutoRegistrar(reg AutoRegistrar) {
	if reg == nil {
		return
	}
	autoRegistrarMu.Lock()
	autoRegistrars = append(autoRegistrars, reg)
	autoRegistrarMu.Unlock()
}

func (e *Engine) applyAutoRegistrars() error {
	if e.autoRegistered {
		return nil
	}
	autoRegistrarMu.RLock()
	list := make([]AutoRegistrar, len(autoRegistrars))
	copy(list, autoRegistrars)
	autoRegistrarMu.RUnlock()
	for _, reg := range list {
		if reg == nil {
			continue
		}
		if err := reg(e); err != nil {
			return err
		}
	}
	e.autoRegistered = true
	return nil
}
