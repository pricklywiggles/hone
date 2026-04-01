package platform

import (
	"fmt"
	"sync"
)

var (
	mu        sync.RWMutex
	platforms = map[string]Platform{}
	hostIndex = map[string]Platform{}
)

// Register adds a platform to the registry. Panics on duplicate name or hostname.
func Register(p Platform) {
	mu.Lock()
	defer mu.Unlock()
	name := p.Name()
	if _, exists := platforms[name]; exists {
		panic("platform already registered: " + name)
	}
	platforms[name] = p
	for _, h := range p.Hostnames() {
		if _, exists := hostIndex[h]; exists {
			panic("hostname already registered: " + h)
		}
		hostIndex[h] = p
	}
}

// Get returns the platform by canonical name, or error if unknown.
func Get(name string) (Platform, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := platforms[name]
	if !ok {
		return nil, fmt.Errorf("unknown platform: %s", name)
	}
	return p, nil
}

// ForHost returns the platform matching the given hostname, or nil.
func ForHost(hostname string) Platform {
	mu.RLock()
	defer mu.RUnlock()
	return hostIndex[hostname]
}

// Defaults returns a map of viper config paths to default URL template values
// for all registered platforms.
func Defaults() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	defaults := make(map[string]string, len(platforms))
	for name, p := range platforms {
		defaults["platforms."+name+".url_template"] = p.URLTemplate()
	}
	return defaults
}
