// Package zar provides support to extract files from ZAR archives.
package zar

import (
	"encoding/binary"
	"fmt"
	"io"

	c "github.com/sourcekris/dclextract/common"
)

const (
	// zarFooterLen is the length of the ZAR file footer, which consists of
	// 4 unknown bytes and a 3-byte magic signature ("PT&").
	zarFooterLen = 7
)

// A temporary struct to hold metadata read from the footer directory.
type zarEntry struct {
	filename string
	compSize uint32
}

// Extract processes a ZAR archive and extracts all contained files.
// The ZAR format is unusual because its file directory is at the end of the
// archive, so it must be read backwards.
func Extract(rs io.ReadSeeker) ([]c.ExtractedFileData, error) {
	// This requires two passes:
	// 1. Read the file directory from the end of the file to build a list of entries.
	// 2. Read the compressed file data from the beginning of the file.

	fileSize, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("ZAR: could not determine file size: %w", err)
	}

	// --- Pass 1: Read metadata from the end of the file. ---
	var entries []zarEntry
	var totalDataSize int64

	// The directory starts just before the 7-byte footer.
	currentPos := fileSize - zarFooterLen

	for currentPos > 0 {
		// The structure of a directory entry when read backwards is:
		// [filename length (1 byte)] [filename (variable)] [compressed size (4 bytes)]

		// Read filename length (1 byte).
		rs.Seek(currentPos-1, io.SeekStart)
		fnLenByte := make([]byte, 1)
		if _, err := io.ReadFull(rs, fnLenByte); err != nil {
			return nil, fmt.Errorf("ZAR: failed to read filename length: %w", err)
		}
		fnSize := int(fnLenByte[0])

		// Read filename.
		fnOffset := currentPos - 1 - int64(fnSize)
		rs.Seek(fnOffset, io.SeekStart)
		filename, err := c.ReadFilename(rs, fnSize)
		if err != nil {
			return nil, fmt.Errorf("ZAR: failed to read filename: %w", err)
		}

		// Read compressed size (4 bytes).
		cSizeOffset := fnOffset - 4
		rs.Seek(cSizeOffset, io.SeekStart)
		cSizeBytes := make([]byte, 4)
		if _, err := io.ReadFull(rs, cSizeBytes); err != nil {
			return nil, fmt.Errorf("ZAR: failed to read compressed size: %w", err)
		}
		compSize := binary.LittleEndian.Uint32(cSizeBytes)

		entries = append(entries, zarEntry{filename: filename, compSize: compSize})
		totalDataSize += int64(compSize)

		// Move to the beginning of this directory entry to process the next one.
		currentPos = cSizeOffset

		// The loop terminates when the total size of compressed data we've found
		// accounts for the remaining space at the start of the file.
		if totalDataSize >= currentPos {
			break
		}
	}

	// The entries were read backwards from the file, so we must reverse the slice.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	// --- Pass 2: Read compressed file data from the beginning. ---
	var allFiles []c.ExtractedFileData
	rs.Seek(0, io.SeekStart)

	for _, entry := range entries {
		// ZAR does not store the decompressed size, so we pass 0.
		decompressedData, err := c.ReadAndDecompressBlastData(rs, entry.compSize, 0)
		if err != nil {
			return allFiles, fmt.Errorf("ZAR: processing data for member '%s': %w", entry.filename, err)
		}

		allFiles = append(allFiles, c.ExtractedFileData{
			Filename:         entry.filename,
			Data:             decompressedData,
			CompressedSize:   entry.compSize,
			DecompressedSize: uint32(len(decompressedData)),
		})
	}

	return allFiles, nil
}
