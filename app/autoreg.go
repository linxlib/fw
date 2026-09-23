package app

import "sync"

type AutoRegistrar func(*Engine) error

var (
	// autoRegistrarsMu 守护 autoRegistrars, 命名与所保护的变量保持一致.
	autoRegistrarsMu sync.RWMutex
	autoRegistrars   []AutoRegistrar
)

func RegisterAutoRegistrar(reg AutoRegistrar) {
	if reg == nil {
		return
	}
	autoRegistrarsMu.Lock()
	autoRegistrars = append(autoRegistrars, reg)
	autoRegistrarsMu.Unlock()
}

func (e *Engine) applyAutoRegistrars() error {
	if e.autoRegistered {
		return nil
	}
	autoRegistrarsMu.RLock()
	list := make([]AutoRegistrar, len(autoRegistrars))
	copy(list, autoRegistrars)
	autoRegistrarsMu.RUnlock()
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
