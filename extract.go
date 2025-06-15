package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JoshVarga/blast" // Added for CMZ decompression
)

// ExtractedFileData holds the data and filename for a single extracted file.
type ExtractedFileData struct {
	Filename         string
	Data             []byte
	CompressedSize   uint32
	DecompressedSize uint32
}

func determineFileType(data []byte) FileType {
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

func readFileMagic(rs io.Reader, expectedMagic []byte) (bytesRead int, err error) {
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

func readFilename(rs io.Reader, fnSize int) (string, error) {
	if fnSize == 0 {
		return "", nil
	}
	filenameBytes := make([]byte, fnSize)
	if _, err := io.ReadFull(rs, filenameBytes); err != nil {
		return "", fmt.Errorf("reading filename: %w", err)
	}
	return string(filenameBytes), nil
}

func readAndDecompressBlastData(rs io.Reader, compSize, decompSize uint32) ([]byte, error) {
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

func extractCMZ(rs io.ReadSeeker) ([]ExtractedFileData, error) {
	var allFiles []ExtractedFileData
	processedAnyFile := false

	for {
		// 1. Read Magic (4 bytes)
		br, err := readFileMagic(rs, Signatures[TypeCMZ])
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

func extractNSK(rs io.ReadSeeker) ([]ExtractedFileData, error) {
	var allFiles []ExtractedFileData
	processedAnyFile := false

	for {
		// 1. Read Member Magic (3 bytes "NSK")
		br, err := readFileMagic(rs, Signatures[TypeNSK])
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
		originalFilename, err := readFilename(rs, fnSize)
		if err != nil {
			return allFiles, fmt.Errorf("NSK: reading member filename for member: %w", err)
		}

		// 4. Read Compressed Data & Decompress (using Blast)
		limitedDataReader := io.LimitReader(rs, int64(compSize))
		decompressedData, err := readAndDecompressBlastData(limitedDataReader, compSize, decompSize)
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
func extractTSC(r io.Reader) (data []byte, filename string, compSize uint32, decompSize uint32, err error) {
	return nil, "", 0, 0, fmt.Errorf("TSC extraction not implemented yet")
}

func extractZAR(r io.Reader) (data []byte, filename string, compSize uint32, decompSizeRead uint32, err error) {
	return nil, "", 0, 0, fmt.Errorf("ZAR extraction not implemented yet")
}

func extract(archivePath string) ([]ExtractedFileData, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []ExtractedFileData
	// var err error // This 'err' would shadow the one from os.Open if not careful

	header := make([]byte, 12)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return nil, err
	}
	fileType := determineFileType(header[:n])
	fmt.Printf("Detected file type: %s\n", fileType)

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	switch fileType {
	case TypeCMZ:
		results, err = extractCMZ(f) // f is *os.File, which is an io.ReadSeeker
	case TypeNSK:
		results, err = extractNSK(f) // f is *os.File, which is an io.ReadSeeker
	case TypeTSC:

	case TypeZAR:

	default:
		return nil, fmt.Errorf("unknown file type for %s", archivePath)
	}

	if err != nil {
		// If results has some items, it means a partial extraction succeeded before an error.
		// The caller (main) can decide what to do. Here, we return partial results + error.
		return results, err
	}
	return results, nil
}

func main() { //nolint:funlen // main function can be longer
	if len(os.Args) != 2 {
		fmt.Println("Usage: extract <filename>")
		os.Exit(1)
	}
	inputFilename := os.Args[1]
	extractedItems, err := extract(inputFilename)

	if err != nil {
		// Print error, but continue if there are partial results to write
		fmt.Fprintln(os.Stderr, "Error during extraction:", err)
		if len(extractedItems) == 0 { // No partial results, so exit
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Attempting to write any partially extracted files...")
	}

	if len(extractedItems) == 0 {
		if err == nil { // No error, but no files found
			fmt.Println("No files found to extract from the archive.")
		}
		os.Exit(0) // Exit if no files, even if there was a non-fatal error reported above
	}

	// Write the extracted files to disk.
	defaultFileCounter := 0
	for i, item := range extractedItems {
		outputDestFilename := item.Filename
		if outputDestFilename == "" {
			base := filepath.Base(inputFilename)
			fileExt := filepath.Ext(base)
			baseName := strings.TrimSuffix(base, fileExt)
			if baseName == "" {
				baseName = "extracted_file"
			}
			outputDestFilename = fmt.Sprintf("%s_%d", baseName, defaultFileCounter)
			if len(extractedItems) == 1 && i == 0 { // Only one file, and it's this one
				outputDestFilename = baseName // Use simpler name if only one nameless file
			}
			defaultFileCounter++
			fmt.Printf("No filename found in archive for item %d, using generated name: %s\n", i+1, outputDestFilename)
		}

		writeErr := os.WriteFile(outputDestFilename, item.Data, 0644)
		if writeErr != nil {
			fmt.Fprintf(os.Stderr, "Error writing to file %s: %v\n", outputDestFilename, writeErr)
			// Optionally, set a flag here to exit with error code later if any write fails
		} else {
			fmt.Printf("Successfully extracted %s (compressed: %d bytes, uncompressed: %d bytes) to %s\n", item.Filename, item.CompressedSize, item.DecompressedSize, outputDestFilename)
		}
	}
}
