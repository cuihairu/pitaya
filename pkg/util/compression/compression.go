package compression

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
)

// MaxInflatedSize caps the output of InflateData to bound memory use when
// inflating untrusted compressed payloads (zip-bomb protection). It matches
// the maximum representable protocol packet size (pkg/conn/codec MaxPacketSize
// = 1<<24), so no legitimate message can exceed it.
const MaxInflatedSize = 1 << 24 // 16MB

func DeflateData(data []byte) ([]byte, error) {
	var bb bytes.Buffer
	z := zlib.NewWriter(&bb)
	_, err := z.Write(data)
	if err != nil {
		return nil, err
	}
	z.Close()
	return bb.Bytes(), nil
}

func InflateData(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	// Read one byte past the cap so an oversized stream is detected instead of
	// silently truncated, and error out rather than allocating without bound.
	out, err := io.ReadAll(io.LimitReader(zr, MaxInflatedSize+1))
	if err != nil {
		return nil, err
	}
	if len(out) > MaxInflatedSize {
		return nil, errors.New("compression: inflated data exceeds size limit")
	}
	return out, nil
}

func IsCompressed(data []byte) bool {
	return len(data) > 2 &&
		(
		// zlib
		(data[0] == 0x78 &&
			(data[1] == 0x9C ||
				data[1] == 0x01 ||
				data[1] == 0xDA ||
				data[1] == 0x5E)) ||
			// gzip
			(data[0] == 0x1F && data[1] == 0x8B))
}
