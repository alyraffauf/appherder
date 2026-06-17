package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// elfHeader builds a minimal little-endian ELF header with the given class
// (1=32-bit, 2=64-bit) and section-header-table fields.
func elfHeader(class byte, shoff uint64, shentsize, shnum uint16) []byte {
	h := make([]byte, 64)
	copy(h, []byte{0x7f, 'E', 'L', 'F'})
	h[4] = class
	h[5] = 1 // little-endian
	if class == 2 {
		binary.LittleEndian.PutUint64(h[40:48], shoff)
		binary.LittleEndian.PutUint16(h[58:60], shentsize)
		binary.LittleEndian.PutUint16(h[60:62], shnum)
	} else {
		binary.LittleEndian.PutUint32(h[32:36], uint32(shoff))
		binary.LittleEndian.PutUint16(h[46:48], shentsize)
		binary.LittleEndian.PutUint16(h[48:50], shnum)
	}
	return h
}

func TestAppImageSquashfsOffset(t *testing.T) {
	for _, class := range []byte{1, 2} {
		got, err := appImageSquashfsOffset(bytes.NewReader(elfHeader(class, 1000, 64, 10)))
		if err != nil {
			t.Fatalf("class %d: %v", class, err)
		}
		if want := int64(1000 + 64*10); got != want {
			t.Fatalf("class %d: offset = %d, want %d", class, got, want)
		}
	}
}

func TestAppImageSquashfsOffsetRejectsNonELF(t *testing.T) {
	if _, err := appImageSquashfsOffset(bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("expected error for non-ELF input")
	}
}

func TestAppImageSquashfsOffsetRejectsType1(t *testing.T) {
	h := elfHeader(2, 1000, 64, 10)
	h[8], h[9], h[10] = 'A', 'I', 1
	if _, err := appImageSquashfsOffset(bytes.NewReader(h)); err == nil {
		t.Fatal("expected error for type-1 AppImage")
	}
}
