// Package cmz implements the extraction of files from CMZ archives.
package cmz

import (
	"encoding/binary"
	"fmt"
	"io"

	c "github.com/sourcekris/dclextract/common"
)

func readCMZMemberMetadata(rs io.Reader) (compSize, decompSize uint32, fnSize int, err error) {
	metadata := make([]byte, 16)
	if _, err = io.ReadFull(rs, metadata); err != nil {
		return 0, 0, 0, fmt.Errorf("reading metadata: %w", err)
	}

	compSize = binary.LittleEndian.Uint32(metadata[0:4])
	decompSize = binary.LittleEndian.Uint32(metadata[4:8])
	fnSize = int(metadata[12])

	if fnSize < 0 {
		return 0, 0, 0, fmt.Errorf("invalid filename size %d", fnSize)
	}
	return compSize, decompSize, fnSize, nil
}

// Extract reads and extracts files from a CMZ archive.
func Extract(rs io.ReadSeeker) ([]c.ExtractedFileData, error) {
	var allFiles []c.ExtractedFileData
	processedAnyFile := false

	for {
		// 1. Read Magic (4 bytes)
		br, err := c.ReadFileMagic(rs, Signatures[TypeCMZ])
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// If we read 0 bytes and have processed files, it's a clean end.
				if br == 0 && processedAnyFile && err == io.EOF {
					return allFiles, nil
				}
				// Otherwise, it's an unexpected end or an empty file on first try.
				if processedAnyFile {
					return allFiles, fmt.Errorf("CMZ: unexpected end of archive while expecting next member header: %w", err)
				}
				// Error on the very first attempt to read a member (or magic mismatch)
				return allFiles, fmt.Errorf("CMZ: failed to read member header: %w", err)
			}
			// Other I/O error
			return allFiles, fmt.Errorf("CMZ: error reading member magic: %w", err)
		}
		// At this point, br should be len(expectedMagic) and err is nil
		processedAnyFile = true

		// 2. Read Metadata
		compSize, decompSize, fnSize, err := readCMZMemberMetadata(rs)
		if err != nil {
			return allFiles, fmt.Errorf("CMZ: reading member metadata: %w", err)
		}

		// 3. Read Filename
		originalFilename, err := readFilename(rs, fnSize)
		if err != nil {
			return allFiles, fmt.Errorf("CMZ: reading member filename: %w", err)
		}

		// 4. Read Compressed Data & Decompress
		limitedDataReader := io.LimitReader(rs, int64(compSize))
		decompressedData, err := readAndDecompressBlastData(limitedDataReader, compSize, decompSize)
		if err != nil {
			return allFiles, fmt.Errorf("CMZ: processing data for member '%s': %w", originalFilename, err)
		}

		allFiles = append(allFiles, ExtractedFileData{
			Filename:         originalFilename,
			Data:             decompressedData,
			CompressedSize:   compSize,
			DecompressedSize: decompSize,
		})
	}
}
