package engine

import (
	"errors"
	"testing"
)

func TestRegisterEngine_and_GetEngine(t *testing.T) {
	const name = "test-reg-get"
	defer UnregisterEngine(name)

	RegisterEngine(name, func(modelPath string) (Engine, error) {
		return NewNoopEngine(modelPath), nil
	})

	eng, err := GetEngine(name, "some/path")
	if err != nil {
		t.Fatalf("GetEngine() error = %v, want nil", err)
	}
	if eng == nil {
		t.Fatal("GetEngine() returned nil engine")
	}
	if got := eng.ModelName(); got != "some/path" {
		t.Errorf("engine.ModelName() = %q, want %q", got, "some/path")
	}
}

func TestGetEngine_notFound(t *testing.T) {
	_, err := GetEngine("nonexistent-engine-xyz", "")
	if err == nil {
		t.Fatal("GetEngine() expected error, got nil")
	}
	var notFound *ErrEngineNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("GetEngine() error type = %T, want *ErrEngineNotFound", err)
	}
	if notFound.Name != "nonexistent-engine-xyz" {
		t.Errorf("ErrEngineNotFound.Name = %q, want %q", notFound.Name, "nonexistent-engine-xyz")
	}
}

func TestHasEngine(t *testing.T) {
	const name = "test-has-engine"
	defer UnregisterEngine(name)

	if HasEngine(name) {
		t.Error("HasEngine() should be false before registration")
	}

	RegisterEngine(name, func(modelPath string) (Engine, error) {
		return NewNoopEngine(modelPath), nil
	})

	if !HasEngine(name) {
		t.Error("HasEngine() should be true after registration")
	}
}

func TestListEngines(t *testing.T) {
	names := []string{"test-list-c", "test-list-a", "test-list-b"}
	for _, n := range names {
		n := n
		defer UnregisterEngine(n)
		RegisterEngine(n, func(modelPath string) (Engine, error) {
			return NewNoopEngine(n), nil
		})
	}

	list := ListEngines()

	// Verify all our test engines appear in sorted order.
	idx := 0
	sorted := []string{"test-list-a", "test-list-b", "test-list-c"}
	for _, entry := range list {
		if idx < len(sorted) && entry == sorted[idx] {
			idx++
		}
	}
	if idx != len(sorted) {
		t.Errorf("ListEngines() did not contain all test engines in sorted order; got %v", list)
	}
}

func TestUnregisterEngine(t *testing.T) {
	const name = "test-unreg"
	RegisterEngine(name, func(modelPath string) (Engine, error) {
		return NewNoopEngine(modelPath), nil
	})

	if !UnregisterEngine(name) {
		t.Error("UnregisterEngine() should return true for a registered engine")
	}
	if UnregisterEngine(name) {
		t.Error("UnregisterEngine() should return false for an already-removed engine")
	}
	if HasEngine(name) {
		t.Error("HasEngine() should be false after unregister")
	}
}

func TestGetConstructor(t *testing.T) {
	const name = "test-get-ctor"
	defer UnregisterEngine(name)

	RegisterEngine(name, func(modelPath string) (Engine, error) {
		return NewNoopEngine(modelPath), nil
	})

	ctor, err := GetConstructor(name)
	if err != nil {
		t.Fatalf("GetConstructor() error = %v, want nil", err)
	}
	if ctor == nil {
		t.Fatal("GetConstructor() returned nil constructor")
	}

	eng, err := ctor("test-path")
	if err != nil {
		t.Fatalf("constructor() error = %v", err)
	}
	if eng.ModelName() != "test-path" {
		t.Errorf("constructed engine ModelName() = %q, want %q", eng.ModelName(), "test-path")
	}

	// Not found case
	_, err = GetConstructor("no-such-engine-ctor")
	if err == nil {
		t.Error("GetConstructor() expected error for unknown engine")
	}
}
