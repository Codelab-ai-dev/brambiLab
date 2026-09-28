package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/jpeg" // DecodeConfig for JPEG
	_ "image/png"  // DecodeConfig for PNG
	"io"
	"net/http"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp" // DecodeConfig for WebP
)

type Kind string

const (
	KindImage    Kind = "image"
	KindVideo    Kind = "video"
	KindResource Kind = "resource"
)

// Limits are configurable; defaults follow web-v1.md §12.1.
type Limits struct {
	Image, Video, Resource int64
	// Large uploads (above this size) are serialized: one at a time.
	LargeThreshold int64
	MaxSide        int
	MaxPixels      int64
	// FreeReserve keeps this much disk free after accepting an upload.
	FreeReserve uint64
}

func DefaultLimits() Limits {
	return Limits{
		Image: 20 << 20, Video: 250 << 20, Resource: 100 << 20,
		LargeThreshold: 20 << 20, MaxSide: 16384, MaxPixels: 50_000_000, FreeReserve: 512 << 20,
	}
}

func (l Limits) For(k Kind) int64 {
	switch k {
	case KindImage:
		return l.Image
	case KindVideo:
		return l.Video
	default:
		return l.Resource
	}
}

// Rejection is a client-facing refusal with its HTTP status and stable code.
type Rejection struct {
	Status  int
	Code    string
	Message string
}

func (r *Rejection) Error() string { return r.Code + ": " + r.Message }

func reject(status int, code, format string, args ...any) *Rejection {
	return &Rejection{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

var extensions = map[string]Kind{
	".jpg": KindImage, ".jpeg": KindImage, ".png": KindImage, ".webp": KindImage,
	".mp4": KindVideo, ".m4v": KindVideo,
	".pdf": KindResource, ".zip": KindResource, ".stl": KindResource,
}

// KindForName picks the size limit before any byte is read. The bytes decide the final type.
func KindForName(name string) (Kind, *Rejection) {
	ext := strings.ToLower(filepath.Ext(name))
	if k, ok := extensions[ext]; ok {
		return k, nil
	}
	switch ext {
	case ".heic", ".heif":
		return "", reject(http.StatusUnsupportedMediaType, "unsupported_type", "HEIC/HEIF no está admitido: exporta la foto como JPEG.")
	case ".svg", ".svgz":
		return "", reject(http.StatusUnsupportedMediaType, "unsupported_type", "SVG no está admitido porque puede ejecutar código; exporta como PNG.")
	case ".gif":
		return "", reject(http.StatusUnsupportedMediaType, "unsupported_type", "GIF no está admitido; usa PNG, WebP o un MP4.")
	}
	return "", reject(http.StatusUnsupportedMediaType, "unsupported_type",
		"Formato no admitido. Imágenes: JPEG, PNG, WebP. Vídeo: MP4. Recursos: PDF, ZIP, STL.")
}

// Detected describes validated bytes.
type Detected struct {
	Kind          Kind
	MIME          string
	Width, Height int
	format        string // jpeg, png, webp for images
}

// Detect identifies the file from its signature and structure (never from Content-Type) and
// checks that it matches the kind announced by the file name.
func Detect(r io.ReaderAt, size int64, expected Kind, lim Limits) (Detected, error) {
	head := make([]byte, 64)
	n, _ := r.ReadAt(head, 0)
	head = head[:n]

	var d Detected
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		d = Detected{Kind: KindImage, MIME: "image/jpeg", format: "jpeg"}
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		d = Detected{Kind: KindImage, MIME: "image/png", format: "png"}
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		d = Detected{Kind: KindImage, MIME: "image/webp", format: "webp"}
	case bytes.HasPrefix(head, []byte("GIF8")):
		return d, reject(http.StatusUnsupportedMediaType, "unsupported_type", "GIF no está admitido; usa PNG, WebP o un MP4.")
	case looksLikeMarkup(head):
		return d, reject(http.StatusUnsupportedMediaType, "unsupported_type", "SVG, HTML y XML no están admitidos porque pueden ejecutar código.")
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		brand := string(head[8:12])
		if isHEIF(brand) {
			return d, reject(http.StatusUnsupportedMediaType, "unsupported_type", "HEIC/HEIF no está admitido: exporta la foto como JPEG.")
		}
		if err := checkMP4(r, size); err != nil {
			return d, err
		}
		d = Detected{Kind: KindVideo, MIME: "video/mp4"}
	case bytes.HasPrefix(head, []byte("%PDF-")):
		d = Detected{Kind: KindResource, MIME: "application/pdf"}
	case bytes.HasPrefix(head, []byte("PK\x03\x04")) || bytes.HasPrefix(head, []byte("PK\x05\x06")):
		d = Detected{Kind: KindResource, MIME: "application/zip"}
	case isSTL(r, size, head):
		d = Detected{Kind: KindResource, MIME: "model/stl"}
	case bytes.HasPrefix(head, []byte("MZ")) || bytes.HasPrefix(head, []byte("\x7fELF")) || bytes.HasPrefix(head, []byte("#!")):
		return d, reject(http.StatusUnsupportedMediaType, "unsupported_type", "Los ejecutables y scripts no están admitidos.")
	default:
		return d, reject(http.StatusUnsupportedMediaType, "unsupported_type",
			"No se reconoce el contenido. Imágenes: JPEG, PNG, WebP. Vídeo: MP4. Recursos: PDF, ZIP, STL.")
	}
	if d.Kind != expected {
		return d, reject(http.StatusUnsupportedMediaType, "unsupported_type", "El contenido del archivo no corresponde a su extensión.")
	}
	if d.Kind == KindImage {
		cfg, _, err := image.DecodeConfig(io.NewSectionReader(r, 0, size))
		if err != nil {
			return d, reject(http.StatusUnprocessableEntity, "invalid_media", "La imagen está dañada o no se puede leer.")
		}
		if err := checkDimensions(cfg.Width, cfg.Height, lim); err != nil {
			return d, err
		}
		d.Width, d.Height = cfg.Width, cfg.Height
	}
	return d, nil
}

func checkDimensions(w, h int, lim Limits) error {
	if w <= 0 || h <= 0 || w > lim.MaxSide || h > lim.MaxSide || int64(w)*int64(h) > lim.MaxPixels {
		return reject(http.StatusUnprocessableEntity, "invalid_media",
			"La imagen mide %d×%d px; el máximo es %d px por lado y %d megapíxeles.", w, h, lim.MaxSide, lim.MaxPixels/1_000_000)
	}
	return nil
}

func looksLikeMarkup(head []byte) bool {
	s := strings.ToLower(strings.TrimLeft(string(head), "\xef\xbb\xbf \t\r\n"))
	for _, p := range []string{"<svg", "<?xml", "<!doctype", "<html", "<script"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func isHEIF(brand string) bool {
	switch brand {
	case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1", "avif", "avis":
		return true
	}
	return false
}

var mp4Brands = map[string]bool{
	"isom": true, "iso2": true, "iso4": true, "iso5": true, "iso6": true, "mp41": true, "mp42": true,
	"avc1": true, "dash": true, "M4V ": true, "M4VP": true, "mmp4": true, "qt  ": false,
}

// checkMP4 walks top-level ISO BMFF boxes without reading payloads: ftyp first with an MP4 brand,
// a moov box present, and every box inside the file. It proves the container, not the codecs.
func checkMP4(r io.ReaderAt, size int64) error {
	bad := func(why string) error {
		return reject(http.StatusUnprocessableEntity, "invalid_media", "El MP4 no es válido (%s). Exporta como MP4 H.264 + AAC con faststart.", why)
	}
	var off int64
	var hasMoov, first = false, true
	hdr := make([]byte, 16)
	for boxes := 0; off < size; boxes++ {
		if boxes > 10000 {
			return bad("demasiadas cajas")
		}
		if _, err := r.ReadAt(hdr[:8], off); err != nil {
			return bad("cabecera truncada")
		}
		boxSize := int64(binary.BigEndian.Uint32(hdr[:4]))
		typ := string(hdr[4:8])
		headerLen := int64(8)
		switch boxSize {
		case 1:
			if _, err := r.ReadAt(hdr[8:16], off+8); err != nil {
				return bad("tamaño extendido truncado")
			}
			boxSize = int64(binary.BigEndian.Uint64(hdr[8:16]))
			headerLen = 16
		case 0:
			boxSize = size - off
		}
		if boxSize < headerLen || off+boxSize > size {
			return bad("caja fuera del archivo")
		}
		if first {
			if typ != "ftyp" || boxSize < 16 {
				return bad("falta ftyp")
			}
			brand := make([]byte, 4)
			if _, err := r.ReadAt(brand, off+8); err != nil || !mp4Brands[string(brand)] {
				return bad(fmt.Sprintf("marca %q no admitida", brand))
			}
			first = false
		}
		if typ == "moov" {
			hasMoov = true
		}
		off += boxSize
	}
	if !hasMoov {
		return bad("falta moov")
	}
	return nil
}

// isSTL accepts binary STL (84-byte header + 50 bytes per triangle) or ASCII "solid … endsolid".
func isSTL(r io.ReaderAt, size int64, head []byte) bool {
	if size >= 84 {
		count := make([]byte, 4)
		if _, err := r.ReadAt(count, 80); err == nil {
			n := int64(binary.LittleEndian.Uint32(count))
			if n > 0 && 84+50*n == size {
				return true
			}
		}
	}
	if !bytes.HasPrefix(bytes.TrimLeft(head, " \t\r\n"), []byte("solid")) {
		return false
	}
	tailLen := int64(1024)
	if size < tailLen {
		tailLen = size
	}
	tail := make([]byte, tailLen)
	if _, err := r.ReadAt(tail, size-tailLen); err != nil && err != io.EOF {
		return false
	}
	return bytes.Contains(tail, []byte("endsolid"))
}
