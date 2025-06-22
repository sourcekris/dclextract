package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcekris/dclextract/cmz"
	"github.com/sourcekris/dclextract/nsk"
	"github.com/sourcekris/dclextract/tsc"
	"github.com/sourcekris/dclextract/zar"

	c "github.com/sourcekris/dclextract/common"
)

func extract(archivePath string) ([]c.ExtractedFileData, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []c.ExtractedFileData

	fileSize, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("could not determine file size: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not seek to start: %w", err)
	}

	// Read header and footer chunks for file type detection.
	header := make([]byte, c.MaxSignatureLength)
	n, readErr := f.Read(header)
	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}
	header = header[:n] // Slice to actual bytes read

	var footer []byte
	if fileSize >= int64(c.MaxSignatureLength) {
		footer = make([]byte, c.MaxSignatureLength)
		n, readAtErr := f.ReadAt(footer, fileSize-int64(c.MaxSignatureLength))
		if readAtErr != nil && readAtErr != io.EOF {
			footer = nil // Non-fatal, clear the footer on error
		} else {
			footer = footer[:n]
		}
	}

	fileType := c.DetermineFileType(header, footer)
	fmt.Printf("Detected file type: %s\n", fileType)

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	switch fileType {
	case c.TypeCMZ:
		results, err = cmz.Extract(f)
	case c.TypeNSK:
		results, err = nsk.Extract(f)
	case c.TypeTSC:
		results, err = tsc.Extract(f)
	case c.TypeZAR:
		results, err = zar.Extract(f)
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
			if len(extractedItems) == 1 && i == 0 { // Only one file, and it's this one.
				outputDestFilename = baseName // Use simpler name if only one nameless file.
			}
			defaultFileCounter++
			fmt.Printf("No filename found in archive for item %d, using generated name: %s\n", i+1, outputDestFilename)
		}

		writeErr := os.WriteFile(outputDestFilename, item.Data, 0644)
		if writeErr != nil {
			fmt.Fprintf(os.Stderr, "Error writing data to file %s: %v\n", outputDestFilename, writeErr)
			// Optionally, set a flag here to exit with error code later if any write fails.
		} else {
			fmt.Printf("Successfully extracted %s (compressed: %d bytes, uncompressed: %d bytes) to %s\n", item.Filename, item.CompressedSize, item.DecompressedSize, outputDestFilename)
		}
	}
}
