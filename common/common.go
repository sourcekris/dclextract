package common

import (
	"bytes"
	"fmt"
	"io"

	"github.com/JoshVarga/blast"
)

// FileType represents a type of compressed file
type FileType int

const (
	// TypeCMZ represents a CMZ compressed file
	TypeCMZ FileType = iota
	// TypeNSK represents a NSK compressed file
	TypeNSK
	// TypeTSC represents a TSC compressed file
	TypeTSC
	// TypeZAR represents a ZAR compressed file
	TypeZAR
	// TypeUnknown represents an unknown file type
	TypeUnknown
)

// Signatures holds the magic byte signatures for each file type
var Signatures = map[FileType][]byte{
	TypeCMZ: []byte{'C', 'l', 'a', 'y'},
	TypeNSK: []byte{'N', 'S', 'K'},
	TypeTSC: []byte{0x65, 0x5D, 0x13, 0x8C, 0x08, 0x01},
	TypeZAR: []byte{'P', 'T', '&'},
}

// String returns the string representation of the FileType
func (ft FileType) String() string {
	switch ft {
	case TypeCMZ:
		return "CMZ"
	case TypeNSK:
		return "NSK"
	case TypeTSC:
		return "TSC"
	case TypeZAR:
		return "ZAR"
	default:
		return "Unknown"
	}
}

// ExtractedFileData holds the data and filename for a single extracted file.
type ExtractedFileData struct {
	Filename         string
	Data             []byte
	CompressedSize   uint32
	DecompressedSize uint32
}

// DetermineFileType checks the provided data against known signatures.
func DetermineFileType(data []byte) FileType {
	switch {
	case bytes.HasPrefix(data, Signatures[TypeCMZ]):
		return TypeCMZ
	case bytes.HasPrefix(data, Signatures[TypeNSK]):
		return TypeNSK
	case bytes.HasPrefix(data, Signatures[TypeTSC]):
		return TypeTSC
	case bytes.HasPrefix(data, Signatures[TypeZAR]):
		return TypeZAR
	default:
		return TypeUnknown
	}
}

// ReadAndDecompressBlastData reads compressed data from the provided io.Reader,
// decompresses it using the blast package, and returns the decompressed data.
func ReadAndDecompressBlastData(rs io.Reader, compSize, decompSize uint32) ([]byte, error) {
	compressedData := make([]byte, compSize)
	if _, err := io.ReadFull(rs, compressedData); err != nil {
		return nil, fmt.Errorf("reading compressed data: %w", err)
	}

	blastReader, err := blast.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("creating blast reader: %w", err)
	}
	defer blastReader.Close() // Ensure reader is closed

	decompressedData := make([]byte, decompSize)
	if n, err := io.ReadFull(blastReader, decompressedData); err != nil {
		return nil, fmt.Errorf("decompressing data (read %d of %d bytes): %w", n, decompSize, err)
	}
	return decompressedData, nil
}

// ReadFilename reads a filename of a specified size from the provided io.Reader.
func ReadFilename(rs io.Reader, fnSize int) (string, error) {
	if fnSize == 0 {
		return "", nil
	}
	filenameBytes := make([]byte, fnSize)
	if _, err := io.ReadFull(rs, filenameBytes); err != nil {
		return "", fmt.Errorf("reading filename: %w", err)
	}
	return string(filenameBytes), nil
}

// ReadFileMagic reads file magic byte header from the provided io.Reader.
func ReadFileMagic(rs io.Reader, expectedMagic []byte) (bytesRead int, err error) {
	magicBuffer := make([]byte, len(expectedMagic))
	n, err := io.ReadFull(rs, magicBuffer)
	if err != nil {
		return n, err // Could be io.EOF, io.ErrUnexpectedEOF, or other I/O error
	}
	if !bytes.Equal(magicBuffer, expectedMagic) {
		return n, fmt.Errorf("magic bytes mismatch: expected %x, got %x", expectedMagic, magicBuffer)
	}
	return n, nil
}
