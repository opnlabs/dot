package store

import (
	"testing"
)

const (
	KEY1           = "test-key"
	KEY2           = "test-key2"
	VALUE1         = "TESTING123"
	VALUE2         = "TESTING234"
	NEWVALUE       = "NEWVALUE"
	NONEXISTINGKEY = "12345"
)

func TestSet(t *testing.T) {
	memStore := NewMemStore()

	err := memStore.Set(KEY1, VALUE1)
	if err != nil {
		t.Error(err, "could not set key")
	}

	err = memStore.Set(KEY1, VALUE2)
	if err != ErrKeyExists {
		t.Error("did not return the key exists error")
	}
}

func TestGet(t *testing.T) {
	memStore := NewMemStore()

	err := memStore.Set(KEY2, VALUE2)
	if err != nil {
		t.Error(err, "could not set key")
	}

	val, err := memStore.Get(KEY2)
	if err != nil {
		t.Error(err)
	}
	if val.(string) != VALUE2 {
		t.Errorf("retrieved value not the same, expected %s got %s", VALUE2, val.(string))
	}
}

func TestGetNonExistingKey(t *testing.T) {
	memStore := NewMemStore()

	_, err := memStore.Get(NONEXISTINGKEY)
	if err != ErrKeyDoesntExist {
		t.Error("did not return key doesn't exist error")
	}
}

func TestPreviousEntries(t *testing.T) {
	memStore := NewMemStore()

	val, err := memStore.Get(KEY1)
	if err != nil {
		t.Error(err)
	}
	if val.(string) != VALUE1 {
		t.Errorf("expected %s, got %s", VALUE1, val.(string))
	}
}

func TestDelete(t *testing.T) {
	memStore := NewMemStore()

	err := memStore.Delete(KEY2)
	if err != nil {
		t.Error(err)
	}
	_, err = memStore.Get(KEY2)
	if err != ErrKeyDoesntExist {
		t.Error("delete did not remove the key")
	}
}

func TestUpdate(t *testing.T) {
	memStore := NewMemStore()
	err := memStore.Update(KEY1, NEWVALUE)
	if err != nil {
		t.Error(err)
	}
	val, err := memStore.Get("test-key")
	if err != nil {
		t.Error(err)
	}
	if val.(string) != NEWVALUE {
		t.Errorf("expected %s, got %s", NEWVALUE, val.(string))
	}
}
