package store

import (
	"testing"
)

func TestSet(t *testing.T) {
	memStore := NewMemStore()

	err := memStore.Set("test-key", "TESTING123")
	if err != nil {
		t.Error(err, "could not set key")
	}

	err = memStore.Set("test-key", "TESTING234")
	if err != ErrKeyExists {
		t.Error("did not return the key exists error")
	}
}

func TestGet(t *testing.T) {
	memStore := NewMemStore()

	err := memStore.Set("test-key2", "TESTING123")
	if err != nil {
		t.Error(err, "could not set key")
	}

	val, err := memStore.Get("test-key2")
	if err != nil {
		t.Error(err)
	}
	if val.(string) != "TESTING123" {
		t.Errorf("retrieved value not the same, expected TESTING123 got %s", val.(string))
	}
}

func TestGetNonExistingKey(t *testing.T) {
	memStore := NewMemStore()

	_, err := memStore.Get("123456")
	if err != ErrKeyDoesntExist {
		t.Error("did not return key doesn't exist error")
	}
}

func TestPreviousEntries(t *testing.T) {
	memStore := NewMemStore()

	val, err := memStore.Get("test-key")
	if err != nil {
		t.Error(err)
	}
	if val.(string) != "TESTING123" {
		t.Errorf("expected TESTING123, got %s", val.(string))
	}
}

func TestDelete(t *testing.T) {
	memStore := NewMemStore()

	err := memStore.Delete("test-key2")
	if err != nil {
		t.Error(err)
	}
	_, err = memStore.Get("test-key2")
	if err != ErrKeyDoesntExist {
		t.Error("delete did not remove the key")
	}
}

func TestUpdate(t *testing.T) {
	memStore := NewMemStore()
	err := memStore.Update("test-key", "NEWVALUE")
	if err != nil {
		t.Error(err)
	}
	val, err := memStore.Get("test-key")
	if err != nil {
		t.Error(err)
	}
	if val.(string) != "NEWVALUE" {
		t.Errorf("expected NEWVALUE, got %s", val.(string))
	}
}
