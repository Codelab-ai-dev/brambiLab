package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"testing"
)

func detectBytes(data []byte, expected Kind) (Detected, error) {
	return Detect(bytes.NewReader(data), int64(len(data)), expected, DefaultLimits())
}

func code(err error) string {
	if r, ok := err.(*Rejection); ok {
		return r.Code
	}
	if err == nil {
		return ""
	}
	return "other"
}

func TestDetectAcceptsTheAllowlist(t *testing.T) {
	cases := []struct {
		file string
		kind Kind
		mime string
	}{
		{"rotated.jpg", KindImage, "image/jpeg"},
		{"text.png", KindImage, "image/png"},
		{"exif.webp", KindImage, "image/webp"},
		{"h264.mp4", KindVideo, "video/mp4"},
		{"vp9.mp4", KindVideo, "video/mp4"},
		{"doc.pdf", KindResource, "application/pdf"},
		{"ascii.stl", KindResource, "model/stl"},
	}
	for _, c := range cases {
		d, err := detectBytes(fixture(t, c.file), c.kind)
		if err != nil || d.MIME != c.mime {
			t.Errorf("%s: %+v %v", c.file, d, err)
		}
	}
	// Binary STL: 80-byte header + count + 50 bytes per triangle.
	stl := make([]byte, 84+50*2)
	binary.LittleEndian.PutUint32(stl[80:84], 2)
	if d, err := detectBytes(stl, KindResource); err != nil || d.MIME != "model/stl" {
		t.Errorf("binary STL: %+v %v", d, err)
	}
	if d, err := detectBytes([]byte("PK\x05\x06"+string(make([]byte, 18))), KindResource); err != nil || d.MIME != "application/zip" {
		t.Errorf("empty zip: %+v %v", d, err)
	}
	if d, _ := detectBytes(fixture(t, "rotated.jpg"), KindImage); d.Width != 64 || d.Height != 32 {
		t.Errorf("stored dimensions: %dx%d", d.Width, d.Height)
	}
}

func TestDetectRejectsDisguisedAndDangerousFiles(t *testing.T) {
	heic := append([]byte{0, 0, 0, 24}, []byte("ftypheic\x00\x00\x00\x00mif1heic")...)
	mov := append([]byte{0, 0, 0, 20}, []byte("ftypqt  \x00\x00\x00\x00qt  ")...)
	mp4NoMoov := append([]byte{0, 0, 0, 16}, []byte("ftypisom\x00\x00\x00\x00")...)
	mp4Overflow := append(append([]byte{}, mp4NoMoov...), 0x7f, 0xff, 0xff, 0xff, 'm', 'o', 'o', 'v')
	cases := []struct {
		name     string
		data     []byte
		expected Kind
		want     string
	}{
		{"html as jpg", []byte("<html><script>alert(1)</script></html>"), KindImage, "unsupported_type"},
		{"svg as png", []byte(`<?xml version="1.0"?><svg onload="alert(1)"/>`), KindImage, "unsupported_type"},
		{"svg with BOM", []byte("\xef\xbb\xbf<svg/>"), KindImage, "unsupported_type"},
		{"gif", []byte("GIF89a" + string(make([]byte, 20))), KindImage, "unsupported_type"},
		{"heic as jpg", heic, KindImage, "unsupported_type"},
		{"exe as pdf", []byte("MZ\x90\x00" + string(make([]byte, 60))), KindResource, "unsupported_type"},
		{"elf as zip", []byte("\x7fELF" + string(make([]byte, 60))), KindResource, "unsupported_type"},
		{"script as stl", []byte("#!/bin/sh\nrm -rf /\n"), KindResource, "unsupported_type"},
		{"pdf named .mp4", fixture(t, "doc.pdf"), KindVideo, "unsupported_type"},
		{"mp4 named .jpg", fixture(t, "h264.mp4"), KindImage, "unsupported_type"},
		{"jpeg named .pdf", fixture(t, "rotated.jpg"), KindResource, "unsupported_type"},
		{"quicktime", mov, KindVideo, "invalid_media"},
		{"mp4 without moov", mp4NoMoov, KindVideo, "invalid_media"},
		{"mp4 box overflow", mp4Overflow, KindVideo, "invalid_media"},
		{"truncated jpeg header", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, KindImage, "invalid_media"},
		{"random bytes", []byte{1, 2, 3, 4, 5, 6, 7, 8}, KindResource, "unsupported_type"},
	}
	for _, c := range cases {
		if _, err := detectBytes(c.data, c.expected); code(err) != c.want {
			t.Errorf("%s: got %v, want %s", c.name, err, c.want)
		}
	}
}

func TestDimensionLimitsStopDecompressionBombs(t *testing.T) {
	// A tiny PNG header declaring 40000×40000 pixels: rejected from DecodeConfig, never decoded.
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1, 1)))
	data := buf.Bytes()
	binary.BigEndian.PutUint32(data[16:20], 40000)
	binary.BigEndian.PutUint32(data[20:24], 40000)
	if _, err := detectBytes(data, KindImage); code(err) != "invalid_media" {
		t.Fatalf("huge declared PNG: %v", err)
	}
	lim := DefaultLimits()
	lim.MaxPixels = 100
	if err := checkDimensions(20, 20, lim); err == nil {
		t.Fatal("pixel limit not enforced")
	}
}

func TestKindForName(t *testing.T) {
	for name, want := range map[string]Kind{"foto.JPG": KindImage, "a.b.webp": KindImage, "v.mp4": KindVideo, "plano.stl": KindResource, "doc.PDF": KindResource} {
		if k, err := KindForName(name); err != nil || k != want {
			t.Errorf("%s: %v %v", name, k, err)
		}
	}
	for _, name := range []string{"foto.heic", "logo.svg", "anim.gif", "virus.exe", "sin-extension", "../../etc/passwd"} {
		if _, err := KindForName(name); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
