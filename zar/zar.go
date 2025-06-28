// Package zar provides support to extract files from ZAR archives.
package zar

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	c "github.com/sourcekris/dclextract/common"
)

const (
	// zarFooterLen is the length of the ZAR file footer, which consists of
	// 4 unknown bytes and a 3-byte magic signature ("PT&").
	zarFooterLen = 7
)

// A temporary struct to hold metadata read from the footer directory.
type zarEntry struct {
	cSize    uint32 // Compressed file size
	fnSize   int    // Filename length
	fnOff    int    // Filename offset
	fnLenOff int    // Filename Length byte offset
	fn       string // Filename for this header.
}

type header struct {
	id    []byte // Stores the file sig
	fSize int    // Total file size
}

// Seeks backwards len bytes, reads forwards, then returns to original position in file.
func readBack(f io.ReadSeeker, len int64) ([]byte, error) {
	buf := make([]byte, len)
	_, err := f.Seek(-len, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	_, err = f.Read(buf)
	if err != nil {
		return nil, err
	}
	_, err = f.Seek(-len, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// fTell returns the current position in the file.
func fTell(f io.ReadSeeker) (int, error) {
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	_, err = f.Seek(pos, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return int(pos), nil
}

// Extract processes a ZAR archive and extracts all contained files.
func Extract(rs io.ReadSeeker) ([]c.ExtractedFileData, error) {

	var (
		bytesRead   int
		headerBytes int
		entries     []*zarEntry
		allFiles    []c.ExtractedFileData
	)

	// Get filesize and initialize the header struct.
	fs, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("ZAR: could not determine file size: %w", err)
	}

	h := &header{
		id:    make([]byte, len(c.Signatures[c.TypeZAR])),
		fSize: int(fs),
	}

	// Read the file id from the end.
	_, err = rs.Seek(-3, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("ZAR: could not seek to the end of the file: %w", err)
	}

	_, err = rs.Read(h.id)
	if err != nil {
		return nil, fmt.Errorf("ZAR: could not read the file magic: %w", err)
	}

	bytesRead = bytesRead + 3

	// Skip backwards 4 bytes past the 2 unknown uint16s.
	_, err = rs.Seek(-7, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("ZAR: could not seek backwards 7 bytes: %w", err)
	}

	bytesRead = bytesRead + 4

	// Read archive entries in a loop.
	for {
		// Read filesize first.
		buf, err := readBack(rs, 4)
		if err != nil {
			return nil, fmt.Errorf("ZAR: could not read compressed size: %w", err)
		}

		e := &zarEntry{
			cSize: binary.LittleEndian.Uint32(buf),
		}

		bytesRead = bytesRead + 4

		// Store current cursor and return to current pos.
		pos, err := fTell(rs)
		if err != nil {
			return nil, fmt.Errorf("ZAR: could not get current position in file: %w", err)
		}

		// Read the filename bytes until we reach the filelen indicator.
		for count := h.fSize - zarFooterLen - headerBytes; ; count-- {
			buf, err := readBack(rs, 1)
			if err != nil {
				return nil, fmt.Errorf("ZAR: could not read filename byte: %w", err)
			}

			bv := int(buf[0]) - 0x80
			pc := int(pos) - count + 4

			if bv == pc {
				e.fnSize = bv
				pos, err = fTell(rs)
				if err != nil {
					return nil, fmt.Errorf("ZAR: could not get position after reading filename size: %w", err)
				}

				e.fnLenOff = pos
				e.fnOff = pos + 1

				headerBytes = headerBytes + e.fnSize + 4 + 1
				bytesRead = bytesRead + e.fnSize + 1 // +1 for the filesize byte itself.
				break
			}
		}

		if e.fnSize > 12 {
			fmt.Fprintf(os.Stderr, "filename length is > 12: %d", e.fnSize)
			os.Exit(1)
		}

		fn := make([]byte, e.fnSize)
		_, err = rs.Seek(int64(e.fnOff), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("ZAR: could not seek to filename offset: %w", err)
		}

		_, err = rs.Read(fn)
		if err != nil {
			return nil, fmt.Errorf("ZAR: could not read filename: %w", err)
		}
		e.fn = string(fn)

		_, err = rs.Seek(int64(e.fnLenOff), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("ZAR: could not seek to filename length offset: %w", err)
		}

		bytesRead = bytesRead + int(e.cSize)
		entries = append(entries, e)

		if bytesRead == h.fSize {
			_, err := rs.Seek(0, io.SeekStart)
			if err != nil {
				return nil, fmt.Errorf("ZAR: could not seek to start of file: %w", err)
			}
			break
		}
	}

	// Reverse the entries slice to ensure we read them in original order.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	for _, entry := range entries {
		// ZAR does not store the decompressed size, so we pass 0.
		decompressedData, err := c.ReadAndDecompressBlastData(rs, entry.cSize, 0)
		if err != nil {
			return allFiles, fmt.Errorf("ZAR: processing data for member '%s': %w", entry.fn, err)
		}

		allFiles = append(allFiles, c.ExtractedFileData{
			Filename:         entry.fn,
			Data:             decompressedData,
			CompressedSize:   entry.cSize,
			DecompressedSize: uint32(len(decompressedData)),
		})
	}

	return allFiles, nil
}
