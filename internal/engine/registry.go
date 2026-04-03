package engine

import (
	"sort"
	"sync"
)

// EngineConstructor is the factory signature stored in the registry.
// modelPath points to the directory containing model artifacts.
type EngineConstructor func(modelPath string) (Engine, error)

// registry is the process-wide engine registry.
var registry = &engineRegistry{
	constructors: make(map[string]EngineConstructor),
}

// engineRegistry holds the mapping from model name to constructor. It is safe
// for concurrent use.
type engineRegistry struct {
	mu           sync.RWMutex
	constructors map[string]EngineConstructor
}

// RegisterEngine adds a named engine constructor to the global registry.
// If a constructor with the same name already exists it is silently replaced.
func RegisterEngine(name string, constructor EngineConstructor) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.constructors[name] = constructor
}

// GetEngine retrieves the constructor for name and invokes it with modelPath,
// returning the ready-to-use Engine. Returns ErrEngineNotFound if the name
// is not registered.
func GetEngine(name string, modelPath string) (Engine, error) {
	registry.mu.RLock()
	ctor, ok := registry.constructors[name]
	registry.mu.RUnlock()

	if !ok {
		return nil, &ErrEngineNotFound{Name: name}
	}
	return ctor(modelPath)
}

// GetConstructor retrieves the raw constructor without invoking it.
// Returns ErrEngineNotFound if the name is not registered.
func GetConstructor(name string) (EngineConstructor, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	ctor, ok := registry.constructors[name]
	if !ok {
		return nil, &ErrEngineNotFound{Name: name}
	}
	return ctor, nil
}

// ListEngines returns the sorted names of all registered engine constructors.
func ListEngines() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.constructors))
	for name := range registry.constructors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasEngine reports whether a constructor is registered under name.
func HasEngine(name string) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	_, ok := registry.constructors[name]
	return ok
}

// UnregisterEngine removes a named engine constructor. It is mainly useful in
// tests. Returns true if the entry existed.
func UnregisterEngine(name string) bool {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	_, ok := registry.constructors[name]
	if ok {
		delete(registry.constructors, name)
	}
	return ok
}
