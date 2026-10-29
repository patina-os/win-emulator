// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"testing"
)

func TestState(t *testing.T) {
	dir := t.TempDir()
	if s, err := loadState(dir); err != nil || s.Phase != phaseNew {
		t.Fatalf("a new directory is phase new, got %+v, %v", s, err)
	}
	if err := (State{Phase: phaseInstalled, Release: "v1"}).save(dir); err != nil {
		t.Fatal(err)
	}
	if s, _ := loadState(dir); s.Phase != phaseInstalled || s.Release != "v1" {
		t.Errorf("state did not round-trip: %+v", s)
	}
}

func TestICO(t *testing.T) {
	var pngs [][]byte
	for _, size := range []int{32, 256} {
		var b bytes.Buffer
		png.Encode(&b, image.NewRGBA(image.Rect(0, 0, size, size)))
		pngs = append(pngs, b.Bytes())
	}
	ico, err := icoFromPNGs(pngs...)
	if err != nil {
		t.Fatal(err)
	}
	var head [3]uint16
	binary.Read(bytes.NewReader(ico), binary.LittleEndian, &head)
	if head != [3]uint16{0, 1, 2} {
		t.Errorf("header %v", head)
	}
	if ico[6] != 32 || ico[6+16] != 0 {
		t.Errorf("widths %d and %d, want 32 and 0 (256)", ico[6], ico[6+16])
	}
	if off := binary.LittleEndian.Uint32(ico[6+12:]); off != 6+32 {
		t.Errorf("first image at %d, want %d", off, 6+32)
	}
}
