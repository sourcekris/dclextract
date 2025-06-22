// Package tsc provides support to extract files from TSC archives.
package tsc

import (
	"encoding/binary"
	"fmt"
	"io"

	c "github.com/sourcekris/dclextract/common"
)

// readTSCMemberHeader reads the 16-byte header for a single member inside a TSC archive.
func readTSCMemberHeader(rs io.Reader) (compSize uint32, fnSize int, err error) {
	header := make([]byte, 16)
	if _, errRead := io.ReadFull(rs, header); errRead != nil {
		// A clean EOF here means we've finished reading all members.
		if errRead == io.EOF {
			return 0, 0, io.EOF
		}
		return 0, 0, fmt.Errorf("reading TSC member header block: %w", errRead)
	}

	// The structure of the member header is inferred from previous implementations.
	compSize = binary.LittleEndian.Uint32(header[1:5])
	fnSize = int(header[15])
	return compSize, fnSize, nil
}

// Extract processes a TSC archive and extracts all contained files.
func Extract(rs io.ReadSeeker) ([]c.ExtractedFileData, error) {
	// 1. Read file-level magic bytes (once)
	if _, err := c.ReadFileMagic(rs, c.Signatures[c.TypeTSC]); err != nil {
		return nil, fmt.Errorf("TSC: reading file magic: %w", err)
	}

	// 2. Read file-level version information (once)
	versionBytes := make([]byte, 3)
	if _, err := io.ReadFull(rs, versionBytes); err != nil {
		return nil, fmt.Errorf("TSC: reading version info: %w", err)
	}
	majorVersion := versionBytes[0]
	minorVersion := binary.LittleEndian.Uint16(versionBytes[1:3])
	versionStr := fmt.Sprintf("%d.%d", majorVersion, minorVersion)

	// 3. Read the wildcard value (1 byte)
	var wildcardValue [1]byte
	if _, err := io.ReadFull(rs, wildcardValue[:]); err != nil {
		return nil, fmt.Errorf("TSC: reading wildcard value: %w", err)
	}

	// 4. Seek past the reserved bytes (4 bytes)
	if _, err := rs.Seek(4, io.SeekCurrent); err != nil {
		return nil, fmt.Errorf("TSC: seeking past reserved bytes: %w", err)
	}

	var allFiles []c.ExtractedFileData

	for {
		// 5. Read the header for the next member
		compSize, fnSize, err := readTSCMemberHeader(rs)
		if err != nil {
			if err == io.EOF {
				// Cleanly reached the end of all members
				break
			}
			return allFiles, fmt.Errorf("TSC: reading member header: %w", err)
		}

		// 6. Read Filename
		originalFilename, err := c.ReadFilename(rs, fnSize+1) // +1 for the null terminator.
		if err != nil {
			return allFiles, fmt.Errorf("TSC: reading member filename for member: %w", err)
		}

		// 7. Read Compressed Data & Decompress (using Blast)
		limitedDataReader := io.LimitReader(rs, int64(compSize))
		decompressedData, err := c.ReadAndDecompressBlastData(limitedDataReader, compSize, 0) // TSC does not provide decompressed size in the member.
		if err != nil {
			return allFiles, fmt.Errorf("TSC: processing data for member '%s': %w", originalFilename, err)
		}

		allFiles = append(allFiles, c.ExtractedFileData{
			Filename:         originalFilename,
			Data:             decompressedData,
			CompressedSize:   compSize,
			DecompressedSize: uint32(len(decompressedData)), // Set size after decompression
			Version:          versionStr,                    // Apply global version to this file
		})
	}
	return allFiles, nil
}
