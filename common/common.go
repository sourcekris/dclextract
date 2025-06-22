package common

import (
	"bytes"
	"fmt"
	"io"
	"time"

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
	TypeTSC: []byte{0x65, 0x5D, 0x13, 0x8C, 0x08},
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
	Version          string
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

	// If decompressed size is not known, read everything until the stream ends.
	if decompSize == 0 {
		decompressedData, err := io.ReadAll(blastReader)
		if err != nil {
			return nil, fmt.Errorf("decompressing data with unknown size: %w", err)
		}
		return decompressedData, nil
	}

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

// ReadDOSModifiedTimeStamp reads a DOS timestamp from the provided io.Reader
// and converts it to a time.Time object.
func ReadDOSModifiedTimeStamp(rs io.Reader) (time.Time, error) {
	var (
		db                     [2]byte
		tb                     [2]byte
		yr, mo, da, hr, mi, se int
	)
	// Read the date and time from the DOS timestamp.
	if _, err := io.ReadFull(rs, db[:]); err != nil {
		return time.Time{}, fmt.Errorf("reading DOS modified date: %w", err)
	}
	if _, err := io.ReadFull(rs, tb[:]); err != nil {
		return time.Time{}, fmt.Errorf("reading DOS modified time: %w", err)
	}

	ddate := uint16(db[0]) | (uint16(db[1]) << 8)
	dtime := uint16(tb[0]) | (uint16(tb[1]) << 8)

	// Convert the DOS date and time to year, month, day, hour, minute, second.
	yr = 1980 + int((ddate&0xfe00)>>9)
	mo = int((ddate & 0x01e0) >> 5)
	da = int(ddate & 0x001f)
	hr = int((dtime & 0xf800) >> 11)
	mi = int((dtime & 0x07e0) >> 5)
	se = int(2 * (dtime & 0x001f))

	return time.Date(yr, time.Month(mo), da, hr, mi, se, 0, time.UTC), nil
}
