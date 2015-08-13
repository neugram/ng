package memframe

import (
	"testing"

	"numgrad.io/frame"
)

func TestMemory(t *testing.T) {
	fm := New(6, 4)
	var f frame.Frame = fm
	if w, h := f.Size(); w != 6 || h != 4 {
		t.Fatalf("Size(): %d, %d want %d, %d", w, h, 6, 4)
	}
	if err := fm.Set(2, 1, 2.1); err != nil {
		t.Errorf("Set error: %v", err)
	}
	v, err := fm.Get(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if v != 2.1 {
		t.Errorf("Get(2, 1) = %v, want %v", v, 2.1)
	}

	f = frame.Slice(f, 2, 3, 1, 1)
	if w, h := f.Size(); w != 3 || h != 1 {
		t.Fatalf("slice Size(): %d, %d want %d, %d", w, h, 3, 1)
	}
	if _, ok := f.(*Memory); !ok {
		t.Fatalf("slice produced wrong type: %T", f)
	}
	v, err = f.Get(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if v != 2.1 {
		t.Errorf("slice Get(0, 0) = %v, want %v", v, 2.1)
	}
}
