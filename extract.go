package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcekris/dclextract/cmz"
	"github.com/sourcekris/dclextract/nsk"

	c "github.com/sourcekris/dclextract/common"
)

func extractTSC(r io.Reader) (data []byte, filename string, compSize uint32, decompSize uint32, err error) {
	return nil, "", 0, 0, fmt.Errorf("TSC extraction not implemented yet")
}

func extractZAR(r io.Reader) (data []byte, filename string, compSize uint32, decompSizeRead uint32, err error) {
	return nil, "", 0, 0, fmt.Errorf("ZAR extraction not implemented yet")
}

func extract(archivePath string) ([]c.ExtractedFileData, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []c.ExtractedFileData
	// var err error // This 'err' would shadow the one from os.Open if not careful

	header := make([]byte, 12)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return nil, err
	}
	fileType := c.DetermineFileType(header[:n])
	fmt.Printf("Detected file type: %s\n", fileType)

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	switch fileType {
	case c.TypeCMZ:
		results, err = cmz.Extract(f) // f is *os.File, which is an io.ReadSeeker
	case c.TypeNSK:
		results, err = nsk.Extract(f) // f is *os.File, which is an io.ReadSeeker
	case c.TypeTSC:

	case c.TypeZAR:

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
