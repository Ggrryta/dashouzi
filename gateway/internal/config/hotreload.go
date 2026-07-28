package config

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher 监听配置文件变化
type Watcher struct {
	mu       sync.RWMutex
	config   *Config
	callback func(*Config)
}

func NewWatcher(path string, callback func(*Config)) (*Watcher, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	w := &Watcher{config: cfg, callback: callback}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(path); err != nil {
		return nil, err
	}

	go func() {
		for range watcher.Events {
			newCfg, err := Load(path)
			if err != nil {
				log.Printf("hot reload failed: %v", err)
				continue
			}
			w.mu.Lock()
			w.config = newCfg
			w.mu.Unlock()
			if w.callback != nil {
				w.callback(newCfg)
			}
			log.Println("config hot reloaded")
		}
	}()

	return w, nil
}

func (w *Watcher) Get() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}
