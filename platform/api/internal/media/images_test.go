package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "golang.org/x/image/webp"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// exifFields runs exiftool (an independent implementation) when available.
func exifFields(t *testing.T, data []byte) string {
	t.Helper()
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Log("exiftool not installed: skipping independent metadata check")
		return ""
	}
	f := filepath.Join(t.TempDir(), "check")
	if err := os.WriteFile(f, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("exiftool", "-s", "-EXIF:all", "-XMP:all", "-IPTC:all", "-PNG:Comment", "-File:Comment", "-GPS:all", f).CombinedOutput()
	if err != nil {
		t.Fatalf("exiftool: %v %s", err, out)
	}
	return string(out)
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0xB000 && g < 0x5000 && b < 0x5000
}

func isBlue(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return b > 0xB000 && r < 0x5000 && g < 0x5000
}

func TestJPEGMetadataRemovedAndOrientationApplied(t *testing.T) {
	src := fixture(t, "rotated.jpg")
	if !strings.Contains(exifFields(t, src)+"GPS", "GPS") {
		t.Fatal("fixture should carry GPS data")
	}
	out, err := SanitizeImage(src, "jpeg", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"TestCam", "Probe", "Privado", "comentario privado", "Exif"} {
		if bytes.Contains(out, []byte(private)) {
			t.Errorf("sanitized JPEG still contains %q", private)
		}
	}
	if fields := exifFields(t, out); strings.Contains(fields, "GPS") || strings.Contains(fields, "Orientation") || strings.Contains(fields, "Make") || strings.Contains(fields, "Creator") {
		t.Fatalf("exiftool still finds metadata:\n%s", fields)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	// Stored 64×32 (red left, blue right) with orientation 6 → displayed 32×64, red on top.
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 64 {
		t.Fatalf("rotated size = %v, want 32×64", b.Size())
	}
	if !isRed(img.At(16, 8)) || !isBlue(img.At(16, 56)) {
		t.Fatalf("pixels not rotated: top=%v bottom=%v", img.At(16, 8), img.At(16, 56))
	}
}

func TestJPEGWithoutRotationIsNotReencoded(t *testing.T) {
	var src bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	_ = jpeg.Encode(&src, img, &jpeg.Options{Quality: 80})
	// Insert a COM segment and an APP1 XMP after SOI.
	com := []byte{0xFF, 0xFE, 0x00, 0x0A, 's', 'e', 'c', 'r', 'e', 't', '!', '!'}
	xmp := append([]byte{0xFF, 0xE1, 0x00, 0x0B}, []byte("http://x\x00")...) // length 2 + 9
	data := append(append(append([]byte{}, src.Bytes()[:2]...), append(com, xmp...)...), src.Bytes()[2:]...)
	// Append trailing data after EOI, like an embedded second image.
	data = append(data, []byte("\xFF\xD8trailing-private-exif")...)
	out, err := SanitizeImage(data, "jpeg", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, src.Bytes()) {
		t.Fatalf("expected the original encoded bytes back (lossless strip); got %d vs %d bytes", len(out), src.Len())
	}
}

func TestPNGTextChunksRemoved(t *testing.T) {
	src := fixture(t, "text.png")
	out, err := SanitizeImage(src, "png", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"comentario privado", "Privado", "tEXt", "iTXt", "zTXt"} {
		if bytes.Contains(out, []byte(private)) {
			t.Errorf("sanitized PNG still contains %q", private)
		}
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("sanitized PNG does not decode: %v", err)
	}
	if !isRed(img.At(4, 4)) || !isBlue(img.At(60, 4)) {
		t.Fatal("PNG pixels changed")
	}
}

func TestWebPExifRemovedAndRotationRejected(t *testing.T) {
	out, err := SanitizeImage(fixture(t, "exif.webp"), "webp", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("TestCam")) || bytes.Contains(out, []byte("EXIF")) {
		t.Fatal("WebP EXIF not removed")
	}
	if fields := exifFields(t, out); strings.Contains(fields, "GPS") || strings.Contains(fields, "Make") {
		t.Fatalf("exiftool still finds metadata:\n%s", fields)
	}
	if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("sanitized WebP does not decode: %v", err)
	}
	_, err = SanitizeImage(fixture(t, "rotated.webp"), "webp", DefaultLimits())
	if r, ok := err.(*Rejection); !ok || r.Code != "invalid_media" {
		t.Fatalf("rotated WebP: %v", err)
	}
}

func TestDamagedImagesRejectedWithoutPanicking(t *testing.T) {
	good := fixture(t, "rotated.jpg")
	for name, data := range map[string][]byte{
		"truncated jpeg": good[:len(good)/2],
		"bad length":     append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}, make([]byte, 10)...),
		"only soi":       {0xFF, 0xD8},
		"png bad crc":    append(append([]byte{}, fixture(t, "text.png")[:30]...), 0, 0, 0, 0),
		"webp bad riff":  append([]byte("RIFF\xff\xff\xff\x7fWEBP"), make([]byte, 20)...),
	} {
		format := "jpeg"
		if strings.HasPrefix(name, "png") {
			format = "png"
		} else if strings.HasPrefix(name, "webp") {
			format = "webp"
		}
		if _, err := SanitizeImage(data, format, DefaultLimits()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestOrientationTransforms(t *testing.T) {
	// 3×2 image with a unique colour per pixel; check where pixel (0,0) lands for each orientation.
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	marker := color.RGBA{255, 0, 0, 255}
	src.Set(0, 0, marker)
	want := map[int]image.Point{2: {2, 0}, 3: {2, 1}, 4: {0, 1}, 5: {0, 0}, 6: {1, 0}, 7: {1, 2}, 8: {0, 2}}
	for o, p := range want {
		got := orient(src, o)
		if got.At(p.X, p.Y) != color.Color(marker) {
			t.Errorf("orientation %d: marker not at %v", o, p)
		}
	}
}
