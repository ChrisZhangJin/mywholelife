package store

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// maxDecompressedBytes caps zstdDecompress output as a decompression-bomb guard
// (T-03-02). It matches bundle's maxTarTotalBytes — every stored .tar is already
// bounded by ValidateAndBrief, so this is defense-in-depth.
const maxDecompressedBytes = 32 << 20

// zstdCompress returns a complete zstd frame for src. EncodeAll frames
// atomically — no Close()-to-flush footgun. SpeedDefault == level 3 (D-01).
func zstdCompress(src []byte) ([]byte, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeAll(src, nil), nil
}

// zstdDecompress inflates a full zstd frame. DecodeAll errors on a truncated or
// corrupt frame (frame content checksum is on by default); WithDecoderMaxMemory
// rejects frames that would decode past the cap.
func zstdDecompress(src []byte) ([]byte, error) {
	dec, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxDecompressedBytes))
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	out, err := dec.DecodeAll(src, nil)
	if err != nil {
		return nil, err
	}
	if len(out) > maxDecompressedBytes {
		return nil, fmt.Errorf("store: decompressed size %d exceeds cap %d", len(out), maxDecompressedBytes)
	}
	return out, nil
}
