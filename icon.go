// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
)

// icoFromPNGs packs PNG images into one .ico file, which Windows reads since Vista:
// the tray, the dialogs and the window icon take that format. A size of 256 is
// written as 0, as the format requires.
func icoFromPNGs(pngs ...[]byte) ([]byte, error) {
	var head, body bytes.Buffer
	binary.Write(&head, binary.LittleEndian, [3]uint16{0, 1, uint16(len(pngs))})
	offset := 6 + 16*len(pngs)
	for _, p := range pngs {
		cfg, err := png.DecodeConfig(bytes.NewReader(p))
		if err != nil {
			return nil, err
		}
		entry := struct {
			W, H, Colors, Reserved uint8
			Planes, Bits           uint16
			Size, Offset           uint32
		}{uint8(cfg.Width), uint8(cfg.Height), 0, 0, 1, 32, uint32(len(p)), uint32(offset)}
		binary.Write(&head, binary.LittleEndian, entry)
		body.Write(p)
		offset += len(p)
	}
	return append(head.Bytes(), body.Bytes()...), nil
}
