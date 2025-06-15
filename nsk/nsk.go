// Package nsk provides functions specific to reading and interpret NSK compressed files.
package nsk

import (
	"encoding/binary"
	"fmt"
	"io"

	c "github.com/sourcekris/dclextract/common"
)

// readNSKMemberMetadata reads the 14-byte metadata block for an NSK member.
// Metadata structure: compSize (4), unknown (5), decompSize (4), fnSize (1)
func readNSKMemberMetadata(rs io.Reader) (compSize, decompSize uint32, fnSize int, err error) {
	metadata := make([]byte, 14) // As per unnsk.go structure
	if _, errRead := io.ReadFull(rs, metadata); errRead != nil {
		return 0, 0, 0, fmt.Errorf("reading nsk metadata block: %w", errRead)
	}

	compSize = binary.LittleEndian.Uint32(metadata[0:4])
	// metadata[4:9] are 5 unknown bytes (indices 4, 5, 6, 7, 8)
	decompSize = binary.LittleEndian.Uint32(metadata[9:13]) // metadata[9], [10], [11], [12]
	fnSize = int(metadata[13])

	if compSize < 0 || decompSize < 0 || fnSize < 0 { // Basic sanity check
		return 0, 0, 0, fmt.Errorf("invalid nsk metadata values: compSize=%d, decompSize=%d, fnSize=%d", compSize, decompSize, fnSize)
	}
	return compSize, decompSize, fnSize, nil
}

func Extract(rs io.ReadSeeker) ([]c.ExtractedFileData, error) {
	var allFiles []c.ExtractedFileData
	processedAnyFile := false

	for {
		// 1. Read Member Magic (3 bytes "NSK")
		br, err := c.ReadFileMagic(rs, c.Signatures[c.TypeNSK])
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// If we read 0 bytes and have processed files, it's a clean end.
				if br == 0 && processedAnyFile && err == io.EOF {
					return allFiles, nil
				}
				// Otherwise, it's an unexpected end or an empty file on first try.
				if processedAnyFile {
					return allFiles, fmt.Errorf("NSK: unexpected end of archive while expecting next member header: %w", err)
				}
				// Error on the very first attempt to read a member (or magic mismatch)
				return allFiles, fmt.Errorf("NSK: failed to read member header: %w", err)
			}
			// Other I/O error
			return allFiles, fmt.Errorf("NSK: error reading member magic: %w", err)
		}

		processedAnyFile = true

		// 2. Read NSK Member Metadata
		compSize, decompSize, fnSize, err := readNSKMemberMetadata(rs)
		if err != nil {
			return allFiles, fmt.Errorf("NSK: reading member metadata: %w", err)
		}

		// 3. Read Filename
		originalFilename, err := c.ReadFilename(rs, fnSize)
		if err != nil {
			return allFiles, fmt.Errorf("NSK: reading member filename for member: %w", err)
		}

		// 4. Read Compressed Data & Decompress (using Blast)
		limitedDataReader := io.LimitReader(rs, int64(compSize))
		decompressedData, err := c.ReadAndDecompressBlastData(limitedDataReader, compSize, decompSize)
		if err != nil {
			return allFiles, fmt.Errorf("NSK: processing data for member '%s': %w", originalFilename, err)
		}

		allFiles = append(allFiles, ExtractedFileData{
			Filename:         originalFilename,
			Data:             decompressedData,
			CompressedSize:   compSize,
			DecompressedSize: decompSize,
		})
	}
}
