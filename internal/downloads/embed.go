package downloads

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

func embedEffectMetadata(path string, metadata EffectMetadata) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return "", false, err
	}

	var out []byte
	format := detectFormat(data)
	switch format {
	case "png":
		out, err = embedPNG(data, payload)
	case "jpeg":
		out, err = embedJPEG(data, payload)
	case "webp":
		out, err = embedWebP(data, payload)
	case "gif":
		out, err = embedGIF(data, payload)
	default:
		return "", false, nil
	}
	if err != nil {
		return format, false, err
	}
	if err := replaceFile(path, out, 0644); err != nil {
		return format, false, err
	}
	return format, true, nil
}

func detectFormat(data []byte) string {
	switch {
	case bytes.HasPrefix(data, pngSignature):
		return "png"
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8:
		return "jpeg"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "webp"
	case bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")):
		return "gif"
	default:
		return ""
	}
}

func embedPNG(data []byte, payload []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, fmt.Errorf("not a PNG file")
	}
	chunks := [][]byte{
		pngChunk("iTXt", iTXtData("lovart.effect.v1", payload)),
		pngChunk("iTXt", iTXtData("parameters", payload)),
	}
	insert := -1
	pos := len(pngSignature)
	for pos+12 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		chunkType := string(data[pos+4 : pos+8])
		end := pos + 8 + length + 4
		if end > len(data) {
			return nil, fmt.Errorf("invalid PNG chunk length")
		}
		if chunkType == "IEND" {
			insert = pos
			break
		}
		pos = end
	}
	if insert < 0 {
		return nil, fmt.Errorf("PNG missing IEND chunk")
	}
	out := make([]byte, 0, len(data)+len(chunks[0])+len(chunks[1]))
	out = append(out, data[:insert]...)
	for _, chunk := range chunks {
		out = append(out, chunk...)
	}
	out = append(out, data[insert:]...)
	return out, nil
}

func iTXtData(keyword string, text []byte) []byte {
	data := make([]byte, 0, len(keyword)+5+len(text))
	data = append(data, keyword...)
	data = append(data, 0) // keyword terminator
	data = append(data, 0) // uncompressed
	data = append(data, 0) // compression method
	data = append(data, 0) // empty language tag
	data = append(data, 0) // empty translated keyword
	data = append(data, text...)
	return data
}

func pngChunk(kind string, data []byte) []byte {
	out := make([]byte, 0, 12+len(data))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))
	out = append(out, length[:]...)
	out = append(out, kind...)
	out = append(out, data...)
	crcInput := append([]byte(kind), data...)
	var crc [4]byte
	binary.BigEndian.PutUint32(crc[:], crc32.ChecksumIEEE(crcInput))
	out = append(out, crc[:]...)
	return out
}

func embedJPEG(data []byte, payload []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return nil, fmt.Errorf("not a JPEG file")
	}
	xmp := xmpPacket(payload)
	appPayload := append([]byte("http://ns.adobe.com/xap/1.0/\x00"), xmp...)
	if len(appPayload)+2 > 65535 {
		return nil, fmt.Errorf("JPEG XMP payload too large")
	}
	segment := make([]byte, 0, 4+len(appPayload))
	segment = append(segment, 0xff, 0xe1)
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(appPayload)+2))
	segment = append(segment, length[:]...)
	segment = append(segment, appPayload...)

	insert := jpegMetadataInsertOffset(data)
	out := make([]byte, 0, len(data)+len(segment))
	out = append(out, data[:insert]...)
	out = append(out, segment...)
	out = append(out, data[insert:]...)
	return out, nil
}

func jpegMetadataInsertOffset(data []byte) int {
	pos := 2
	for pos+4 <= len(data) && data[pos] == 0xff {
		marker := data[pos+1]
		if !((marker >= 0xe0 && marker <= 0xef) || marker == 0xfe) {
			break
		}
		length := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if length < 2 || pos+2+length > len(data) {
			break
		}
		pos += 2 + length
	}
	return pos
}

func embedWebP(data []byte, payload []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, fmt.Errorf("not a WebP file")
	}
	xmp := xmpPacket(payload)
	chunk := make([]byte, 0, 8+len(xmp)+1)
	chunk = append(chunk, 'X', 'M', 'P', ' ')
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(xmp)))
	chunk = append(chunk, size[:]...)
	chunk = append(chunk, xmp...)
	if len(xmp)%2 == 1 {
		chunk = append(chunk, 0)
	}

	out := append([]byte(nil), data...)
	setWebPMetadataFlag(out)
	out = append(out, chunk...)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, nil
}

func setWebPMetadataFlag(data []byte) {
	pos := 12
	for pos+8 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		dataStart := pos + 8
		dataEnd := dataStart + size
		if dataEnd > len(data) {
			return
		}
		if string(data[pos:pos+4]) == "VP8X" && size >= 10 {
			data[dataStart] |= 0x04
			return
		}
		pos = dataEnd
		if size%2 == 1 {
			pos++
		}
	}
}

func embedGIF(data []byte, payload []byte) ([]byte, error) {
	if !(bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))) {
		return nil, fmt.Errorf("not a GIF file")
	}
	trailer := bytes.LastIndexByte(data, 0x3b)
	if trailer < 0 {
		return nil, fmt.Errorf("GIF missing trailer")
	}
	comment := gifComment(append([]byte("LOVART_EFFECT_V1 "), payload...))
	out := make([]byte, 0, len(data)+len(comment))
	out = append(out, data[:trailer]...)
	out = append(out, comment...)
	out = append(out, data[trailer:]...)
	return out, nil
}

func gifComment(text []byte) []byte {
	out := []byte{0x21, 0xfe}
	for len(text) > 0 {
		n := len(text)
		if n > 255 {
			n = 255
		}
		out = append(out, byte(n))
		out = append(out, text[:n]...)
		text = text[n:]
	}
	out = append(out, 0)
	return out
}

func xmpPacket(payload []byte) []byte {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, payload)
	packet := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>` +
		`<x:xmpmeta xmlns:x="adobe:ns:meta/">` +
		`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
		`<rdf:Description xmlns:lovart="https://github.com/aporicho/lovart/ns/1.0/" lovart:Effect="` +
		escaped.String() +
		`"/></rdf:RDF></x:xmpmeta><?xpacket end="w"?>`
	return []byte(packet)
}

func replaceFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
