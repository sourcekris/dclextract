package main

import (
	"bytes"
	"encoding/binary"
	"errors"
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
	case bytes.HasPrefix(data, Signatures[TypeUNCMZ]):
		return TypeUNCMZ
	case bytes.HasPrefix(data, Signatures[TypeUNNSK]):
		return TypeUNNSK
	case bytes.HasPrefix(data, Signatures[TypeUNTSC]):
		return TypeUNTSC
	case bytes.HasPrefix(data, Signatures[TypeUNZAR]):
		return TypeUNZAR
	default:
		return TypeUnknown
	}
}

func extractUNCMZ(rs io.ReadSeeker) ([]ExtractedFileData, error) {
	var allFiles []ExtractedFileData
	processedAnyFile := false

	for {
		currentPos, err := rs.Seek(0, io.SeekCurrent)
		if err != nil {
			return allFiles, fmt.Errorf("UNCMZ: failed to get current position: %w", err)
		}

		// 1. Read Magic (4 bytes)
		magic := make([]byte, len(Signatures[TypeUNCMZ]))
		bytesRead, err := io.ReadFull(rs, magic)

		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// If we read 0 bytes and have processed files, it's a clean end.
				if bytesRead == 0 && processedAnyFile && err == io.EOF {
					return allFiles, nil
				}
				// Otherwise, it's an unexpected end or an empty file on first try.
				if processedAnyFile { // Error trying to read the *next* member
					return allFiles, fmt.Errorf("UNCMZ: unexpected end of archive while expecting next member header: %w", err)
				}
				// Error on the very first attempt to read a member
				return allFiles, fmt.Errorf("UNCMZ: failed to read member header: %w", err)
			}
			// Other I/O error
			return allFiles, fmt.Errorf("UNCMZ: error reading member magic: %w", err)
		}

		if !bytes.Equal(magic, Signatures[TypeUNCMZ]) {
			if !processedAnyFile {
				// This should ideally be caught by determineFileType if the file starts with non-CMZ
				return nil, errors.New("invalid UNCMZ magic at start of stream")
			}
			// Not a CMZ signature, means previous CMZ member was the last. Seek back.
			if _, seekErr := rs.Seek(currentPos, io.SeekStart); seekErr != nil {
				// Log or handle: failed to rewind, but we have extracted files.
				fmt.Fprintf(os.Stderr, "Warning: UNCMZ: failed to seek back after non-CMZ data: %v\n", seekErr)
			}
			return allFiles, nil // End of CMZ members
		}

		processedAnyFile = true

		// 2. Read Metadata (16 bytes)
		metadata := make([]byte, 16)
		if _, err := io.ReadFull(rs, metadata); err != nil {
			return allFiles, fmt.Errorf("UNCMZ: reading metadata for member: %w", err)
		}

		compSize := binary.LittleEndian.Uint32(metadata[0:4])
		decompSize := binary.LittleEndian.Uint32(metadata[4:8])
		fnSize := int(metadata[12])

		if fnSize < 0 {
			return allFiles, fmt.Errorf("UNCMZ: invalid filename size %d for member", fnSize)
		}

		// 3. Read Filename
		var originalFilename string
		if fnSize > 0 {
			filenameBytes := make([]byte, fnSize)
			if _, err := io.ReadFull(rs, filenameBytes); err != nil {
				return allFiles, fmt.Errorf("UNCMZ: reading filename for member: %w", err)
			}
			originalFilename = string(filenameBytes)
		}

		// 4. Read Compressed Data
		compressedData := make([]byte, compSize)
		if _, err := io.ReadFull(rs, compressedData); err != nil {
			return allFiles, fmt.Errorf("UNCMZ: reading compressed data for member '%s': %w", originalFilename, err)
		}

		// 5. Decompress
		blastReader, err := blast.NewReader(bytes.NewReader(compressedData))
		if err != nil {
			return allFiles, fmt.Errorf("UNCMZ: creating blast reader for member '%s': %w", originalFilename, err)
		}

		decompressedData := make([]byte, decompSize)
		if n, err := io.ReadFull(blastReader, decompressedData); err != nil {
			return allFiles, fmt.Errorf("UNCMZ: decompressing data for member '%s' (read %d of %d bytes): %w", originalFilename, n, decompSize, err)
		}
		if err := blastReader.Close(); err != nil {
			// Log this, but don't necessarily fail the whole extraction for it
			fmt.Fprintf(os.Stderr, "Warning: UNCMZ: error closing blast reader for member '%s': %v\n", originalFilename, err)
		}

		allFiles = append(allFiles, ExtractedFileData{
			Filename:         originalFilename,
			Data:             decompressedData,
			CompressedSize:   compSize,
			DecompressedSize: decompSize,
		})
	}
}

func extractUNNSK(r io.Reader) (data []byte, filename string, compSize uint32, decompSize uint32, err error) {
	var header struct {
		Magic      [4]byte
		DecompSize uint32
		CompSize   uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, "", 0, 0, err
	}

	if !bytes.Equal(header.Magic[:], Signatures[TypeUNNSK]) {
		return nil, "", 0, 0, errors.New("invalid UNNSK magic")
	}

	compressedData := make([]byte, header.CompSize)
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return nil, "", 0, 0, err
	}

	blastReader, err := blast.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, "", 0, 0, fmt.Errorf("creating UNNSK blast reader: %w", err)
	}

	decompressedData := make([]byte, header.DecompSize)
	if n, err := io.ReadFull(blastReader, decompressedData); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, "", 0, 0, fmt.Errorf("decompressing UNNSK data (read %d of %d bytes): unexpected EOF: %w", n, header.DecompSize, err)
		}
		return nil, "", 0, 0, fmt.Errorf("decompressing UNNSK data (read %d of %d bytes): %w", n, header.DecompSize, err)
	}
	return decompressedData, "", header.CompSize, header.DecompSize, nil
}

func extractUNTSC(r io.Reader) (data []byte, filename string, compSize uint32, decompSize uint32, err error) {
	var header struct {
		Magic      [4]byte
		DecompSize uint32
		CompSize   uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, "", 0, 0, err
	}

	if !bytes.Equal(header.Magic[:], Signatures[TypeUNTSC]) {
		return nil, "", 0, 0, errors.New("invalid UNTSC magic")
	}

	compressedData := make([]byte, header.CompSize)
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return nil, "", 0, 0, err
	}

	blastReader, err := blast.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, "", 0, 0, fmt.Errorf("creating UNTSC blast reader: %w", err)
	}

	decompressedData := make([]byte, header.DecompSize)
	if n, err := io.ReadFull(blastReader, decompressedData); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, "", 0, 0, fmt.Errorf("decompressing UNTSC data (read %d of %d bytes): unexpected EOF: %w", n, header.DecompSize, err)
		}
		return nil, "", 0, 0, fmt.Errorf("decompressing UNTSC data (read %d of %d bytes): %w", n, header.DecompSize, err)
	}
	return decompressedData, "", header.CompSize, header.DecompSize, nil
}

func extractUNZAR(r io.Reader) (data []byte, filename string, compSize uint32, decompSizeRead uint32, err error) {
	// Ensure we read the full length of the UNZAR signature
	sigLen := len(Signatures[TypeUNZAR])
	if sigLen == 0 { // Should not happen with current signatures.go
		return nil, "", 0, 0, errors.New("UNZAR signature is undefined or empty")
	}
	magic := make([]byte, sigLen)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, "", 0, 0, fmt.Errorf("reading UNZAR magic: %w", err)
	}
	if !bytes.Equal(magic, Signatures[TypeUNZAR]) {
		return nil, "", 0, 0, errors.New("invalid UNZAR magic")
	}

	// var decompSize uint32 // Renamed to decompSizeRead to avoid conflict with return var
	if err = binary.Read(r, binary.LittleEndian, &decompSizeRead); err != nil {
		return nil, "", 0, 0, fmt.Errorf("reading UNZAR decompressed size: %w", err)
	}

	compressedData, err := io.ReadAll(r)
	if err != nil {
		return nil, "", 0, 0, fmt.Errorf("reading UNZAR compressed data: %w", err)
	}

	blastReader, err := blast.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, "", 0, 0, fmt.Errorf("creating UNZAR blast reader: %w", err)
	}

	decompressedData := make([]byte, decompSizeRead)
	if n, err := io.ReadFull(blastReader, decompressedData); err != nil { // Use decompSizeRead here
		if err == io.ErrUnexpectedEOF {
			return nil, "", 0, 0, fmt.Errorf("decompressing UNZAR data (read %d of %d bytes): unexpected EOF: %w", n, decompSizeRead, err)
		}
		return nil, "", 0, 0, fmt.Errorf("decompressing UNZAR data (read %d of %d bytes): %w", n, decompSizeRead, err)
	}
	return decompressedData, "", uint32(len(compressedData)), decompSizeRead, nil
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
	case TypeUNCMZ:
		results, err = extractUNCMZ(f) // f is *os.File, which is an io.ReadSeeker
	case TypeUNNSK:
		data, fname, cSize, uSize, extractErr := extractUNNSK(f)
		if extractErr == nil {
			results = append(results, ExtractedFileData{
				Filename:         fname,
				Data:             data,
				CompressedSize:   cSize,
				DecompressedSize: uSize,
			})
		}
		err = extractErr
	case TypeUNTSC:
		data, fname, cSize, uSize, extractErr := extractUNTSC(f)
		if extractErr == nil {
			results = append(results, ExtractedFileData{
				Filename:         fname,
				Data:             data,
				CompressedSize:   cSize,
				DecompressedSize: uSize,
			})
		}
		err = extractErr
	case TypeUNZAR:
		data, fname, cSize, uSize, extractErr := extractUNZAR(f)
		if extractErr == nil {
			results = append(results, ExtractedFileData{
				Filename:         fname,
				Data:             data,
				CompressedSize:   cSize,
				DecompressedSize: uSize,
			})
		}
		err = extractErr
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
