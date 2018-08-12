package main

import (
	"os"
	"os/signal"
	"sync"
)

type criticalOperationRunner interface {
	runCritical(op func())
}

type signalListener struct {
	critOpMutex sync.RWMutex
}

func (self *signalListener) runCritical(op func()) {
	self.critOpMutex.RLock()
	op()
	self.critOpMutex.RUnlock()
}

func (self *signalListener) onSignals(handler func(sig os.Signal), sigs ...os.Signal) {
	ch := make(chan os.Signal, 64)
	signal.Notify(ch, sigs...)

	go func() {
		for {
			if sig, hasSig := <-ch; hasSig {
				self.critOpMutex.Lock()
				handler(sig)
				self.critOpMutex.Unlock()
			} else {
				break
			}
		}
	}()
}
