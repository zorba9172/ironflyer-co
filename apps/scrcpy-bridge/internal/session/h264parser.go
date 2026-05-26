// Package session owns the scrcpy ↔ Pion ↔ WebSocket lifecycle. The
// h264parser file isolates the Annex-B stream parser used to turn
// scrcpy's stdout into individual NAL units that Pion's
// TrackLocalStaticSample can transmit.
package session

import (
	"bytes"
	"fmt"
	"io"
)

// nalParseBufferSize bounds how much unflushed prefix the parser keeps
// while hunting for the next start code. 1 MiB is generous: a single
// IDR NAL from a 1080p scrcpy stream sits comfortably under that.
const nalParseBufferSize = 1 << 20

// readChunkSize is the per-Read budget the parser pulls from scrcpy.
// 64 KiB matches the "ring buffer" hint without bottlenecking the
// pipeline when scrcpy emits a large IDR.
const readChunkSize = 64 << 10

// ParseNALUs reads an Annex-B H.264 byte stream from r and invokes
// emit once per NAL unit. The NAL unit payload passed to emit excludes
// the start code prefix. The function returns when r reports EOF or
// any other error (io.EOF is converted to nil so the caller can treat
// graceful EOS as a non-error). The slice handed to emit is owned by
// ParseNALUs and reused across calls — emit must copy if it needs to
// retain the bytes past the callback boundary.
func ParseNALUs(r io.Reader, emit func(nalu []byte)) error {
	if r == nil {
		return fmt.Errorf("nil reader")
	}
	buf := make([]byte, 0, nalParseBufferSize)
	read := make([]byte, readChunkSize)
	// startCode4 is the canonical 0x00000001 Annex-B prefix; the
	// shorter 0x000001 form is also legal and scrcpy emits it for
	// some non-IDR slices. We accept both.
	startCode4 := []byte{0x00, 0x00, 0x00, 0x01}
	startCode3 := []byte{0x00, 0x00, 0x01}

	flushUpTo := func(end int) {
		if end <= 0 {
			return
		}
		// Trim any trailing 0x00 padding that immediately precedes
		// the next start code we just located. Annex-B allows zero
		// padding between NAL units.
		payload := buf[:end]
		for len(payload) > 0 && payload[len(payload)-1] == 0x00 {
			payload = payload[:len(payload)-1]
		}
		if len(payload) > 0 {
			emit(payload)
		}
	}

	for {
		n, err := r.Read(read)
		if n > 0 {
			buf = append(buf, read[:n]...)
			// Scan repeatedly so a single Read that contains
			// multiple NAL units is fully drained before we go
			// back for more bytes.
			for {
				idx4 := bytes.Index(buf, startCode4)
				idx3 := bytes.Index(buf, startCode3)
				// Pick the earlier of the two boundaries. A
				// 4-byte code is also a 3-byte code with a
				// leading zero, so we must dedupe.
				var idx, codeLen int
				switch {
				case idx4 == -1 && idx3 == -1:
					idx = -1
				case idx4 == -1:
					idx = idx3
					codeLen = 3
				case idx3 == -1:
					idx = idx4
					codeLen = 4
				default:
					if idx4 <= idx3+1 {
						idx = idx4
						codeLen = 4
					} else {
						idx = idx3
						codeLen = 3
					}
				}
				if idx == -1 {
					// No start code visible yet. Keep
					// reading. Guard the buffer against
					// runaway growth from malformed
					// input.
					if len(buf) > nalParseBufferSize {
						buf = buf[len(buf)-nalParseBufferSize:]
					}
					break
				}
				if idx == 0 {
					// First start code in the stream:
					// nothing to flush yet, just drop
					// the prefix and keep scanning for
					// the next boundary.
					buf = buf[codeLen:]
					continue
				}
				// Bytes before the start code are the
				// previous NAL unit. Emit it, then discard
				// up to (but not including) the start code
				// we're about to skip.
				flushUpTo(idx)
				buf = buf[idx+codeLen:]
			}
		}
		if err != nil {
			if err == io.EOF {
				// Emit whatever tail bytes remain as a
				// final NAL unit.
				if len(buf) > 0 {
					flushUpTo(len(buf))
				}
				return nil
			}
			return err
		}
	}
}
