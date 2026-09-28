package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
)

// SanitizeImage removes metadata (EXIF with GPS/camera data, XMP, IPTC, comments, text chunks)
// and applies the EXIF orientation to the pixels, so nothing private or misleading reaches the
// public file (web-v1.md §12.1). Input is at most the image limit (20 MiB), so it is held in memory.
func SanitizeImage(data []byte, format string, lim Limits) ([]byte, error) {
	switch format {
	case "jpeg":
		return sanitizeJPEG(data, lim)
	case "png":
		return sanitizePNG(data, lim)
	case "webp":
		return sanitizeWebP(data)
	}
	return nil, reject(http.StatusUnsupportedMediaType, "unsupported_type", "Formato de imagen no admitido.")
}

func damaged() error {
	return reject(http.StatusUnprocessableEntity, "invalid_media", "La imagen está dañada o no se puede leer.")
}

// --- JPEG ---------------------------------------------------------------------------------------

func sanitizeJPEG(data []byte, lim Limits) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, damaged()
	}
	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(data[:2])
	var icc [][]byte
	orientation := 1
	pos := 2
	for {
		if pos >= len(data) || data[pos] != 0xFF {
			return nil, damaged()
		}
		for pos < len(data) && data[pos] == 0xFF { // fill bytes
			pos++
		}
		if pos >= len(data) {
			return nil, damaged()
		}
		marker := data[pos]
		segStart := pos - 1
		pos++
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			out.Write(data[segStart:pos])
			continue
		}
		if marker == 0xD9 {
			return nil, damaged() // EOI before any scan
		}
		if pos+2 > len(data) {
			return nil, damaged()
		}
		length := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if length < 2 || pos+length > len(data) {
			return nil, damaged()
		}
		payload := data[pos+2 : pos+length]
		segEnd := pos + length

		if marker == 0xDA { // start of scan: entropy data follows until the first EOI
			end := bytes.Index(data[segEnd:], []byte{0xFF, 0xD9})
			if end < 0 {
				return nil, damaged()
			}
			// Anything after EOI (e.g. an appended second image with its own EXIF) is dropped.
			out.Write(data[segStart : segEnd+end+2])
			break
		}
		switch {
		case marker == 0xE1: // EXIF or XMP: dropped, orientation remembered
			if bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
				orientation = tiffOrientation(payload[6:])
			}
		case marker == 0xE2 && bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00")):
			icc = append(icc, data[segStart:segEnd])
			out.Write(data[segStart:segEnd])
		case marker == 0xE0 || marker == 0xEE: // JFIF, Adobe (colour transform): kept
			out.Write(data[segStart:segEnd])
		case marker >= 0xE2 && marker <= 0xEF, marker == 0xFE: // other APPn (IPTC, MPF…), comments
		default: // tables and frame headers
			out.Write(data[segStart:segEnd])
		}
		pos = segEnd
	}
	clean := out.Bytes()
	if orientation <= 1 || orientation > 8 {
		return clean, nil
	}
	img, err := jpeg.Decode(bytes.NewReader(clean))
	if err != nil {
		return nil, damaged()
	}
	if _, cmyk := img.(*image.CMYK); cmyk {
		return nil, reject(http.StatusUnprocessableEntity, "invalid_media", "Los JPEG CMYK con rotación EXIF no están admitidos; exporta en RGB.")
	}
	rotated := orient(img, orientation)
	if err := checkDimensions(rotated.Bounds().Dx(), rotated.Bounds().Dy(), lim); err != nil {
		return nil, err
	}
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, rotated, &jpeg.Options{Quality: 92}); err != nil {
		return nil, err
	}
	// Re-insert the colour profile right after SOI.
	final := bytes.NewBuffer(make([]byte, 0, enc.Len()+len(icc)*1024))
	final.Write(enc.Bytes()[:2])
	for _, seg := range icc {
		final.Write(seg)
	}
	final.Write(enc.Bytes()[2:])
	return final.Bytes(), nil
}

// tiffOrientation reads tag 0x0112 from IFD0 of a TIFF/EXIF block; 1 when absent or invalid.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:8]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd : ifd+2]))
	for i := 0; i < n && i < 1000; i++ {
		e := ifd + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 && bo.Uint16(t[e+2:e+4]) == 3 {
			v := int(bo.Uint16(t[e+8 : e+10]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient applies an EXIF orientation (2-8) to the pixels.
func orient(src image.Image, o int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, color.RGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)))
		}
	}
	return dst
}

// --- PNG ----------------------------------------------------------------------------------------

var pngDropped = map[string]bool{"eXIf": true, "tEXt": true, "zTXt": true, "iTXt": true, "tIME": true}

func sanitizePNG(data []byte, lim Limits) ([]byte, error) {
	const sig = "\x89PNG\r\n\x1a\n"
	if len(data) < 8 || string(data[:8]) != sig {
		return nil, damaged()
	}
	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.WriteString(sig)
	orientation := 1
	pos, sawIEND := 8, false
	for pos < len(data) && !sawIEND {
		if pos+12 > len(data) {
			return nil, damaged()
		}
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		typ := string(data[pos+4 : pos+8])
		end := pos + 12 + length
		if length < 0 || end > len(data) {
			return nil, damaged()
		}
		if crc32.ChecksumIEEE(data[pos+4:pos+8+length]) != binary.BigEndian.Uint32(data[pos+8+length:end]) {
			return nil, damaged()
		}
		if typ == "eXIf" {
			orientation = tiffOrientation(data[pos+8 : pos+8+length])
		}
		if !pngDropped[typ] {
			out.Write(data[pos:end])
		}
		sawIEND = typ == "IEND"
		pos = end
	}
	if !sawIEND {
		return nil, damaged()
	}
	if orientation <= 1 {
		return out.Bytes(), nil
	}
	img, err := png.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		return nil, damaged()
	}
	rotated := orient(img, orientation)
	if err := checkDimensions(rotated.Bounds().Dx(), rotated.Bounds().Dy(), lim); err != nil {
		return nil, err
	}
	nrgba := image.NewNRGBA(rotated.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), rotated, image.Point{}, draw.Src)
	var enc bytes.Buffer
	if err := png.Encode(&enc, nrgba); err != nil {
		return nil, err
	}
	return enc.Bytes(), nil
}

// --- WebP ---------------------------------------------------------------------------------------

func sanitizeWebP(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, damaged()
	}
	riffEnd := 8 + int(binary.LittleEndian.Uint32(data[4:8]))
	if riffEnd > len(data) || riffEnd < 12 {
		return nil, damaged()
	}
	body := bytes.NewBuffer(make([]byte, 0, len(data)))
	body.WriteString("WEBP")
	pos := 12
	vp8xAt := -1
	for pos < riffEnd {
		if pos+8 > riffEnd {
			return nil, damaged()
		}
		fourcc := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		padded := size + size%2
		end := pos + 8 + padded
		if size < 0 || pos+8+size > riffEnd || end > riffEnd+1 {
			return nil, damaged()
		}
		if end > riffEnd {
			end = riffEnd
		}
		switch fourcc {
		case "EXIF":
			exif := data[pos+8 : pos+8+size]
			exif = bytes.TrimPrefix(exif, []byte("Exif\x00\x00"))
			if tiffOrientation(exif) > 1 {
				return nil, reject(http.StatusUnprocessableEntity, "invalid_media",
					"Esta WebP depende de una rotación EXIF que no se puede aplicar; expórtala ya girada o como JPEG.")
			}
		case "XMP ":
		default:
			if fourcc == "VP8X" {
				vp8xAt = body.Len() + 8
			}
			body.Write(data[pos:end])
		}
		pos = end
	}
	out := body.Bytes()
	if vp8xAt >= 0 && vp8xAt < len(out) {
		out[vp8xAt] &^= 0x08 | 0x04 // EXIF and XMP flags
	}
	final := make([]byte, 8, 8+len(out))
	copy(final, "RIFF")
	binary.LittleEndian.PutUint32(final[4:8], uint32(len(out)))
	return append(final, out...), nil
}
