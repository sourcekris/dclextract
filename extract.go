package main

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

func determineFileType(data []byte) FileType {
	if len(data) < 4 {
		return TypeUnknown
	}
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

func extractUNCMZ(r io.Reader) ([]byte, error) {
	var header struct {
		Magic      [4]byte
		Unknown1   uint32
		Unknown2   uint32
		Unknown3   uint32
		DecompSize uint32
		CompSize   uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if !bytes.Equal(header.Magic[:], Signatures[TypeUNCMZ]) {
		return nil, errors.New("invalid UNCMZ magic")
	}

	compressedData := make([]byte, header.CompSize)
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return nil, err
	}

	reader := flate.NewReader(bytes.NewReader(compressedData))
	decompressedData := make([]byte, header.DecompSize)
	if _, err := io.ReadFull(reader, decompressedData); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, errors.New("decompression failed: unexpected EOF")
		}
		return nil, err
	}
	return decompressedData, nil
}

func extractUNNSK(r io.Reader) ([]byte, error) {
	var header struct {
		Magic      [4]byte
		DecompSize uint32
		CompSize   uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if !bytes.Equal(header.Magic[:], Signatures[TypeUNNSK]) {
		return nil, errors.New("invalid UNNSK magic")
	}

	compressedData := make([]byte, header.CompSize)
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return nil, err
	}

	reader := flate.NewReader(bytes.NewReader(compressedData))
	decompressedData := make([]byte, header.DecompSize)
	if _, err := io.ReadFull(reader, decompressedData); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, errors.New("decompression failed: unexpected EOF")
		}
		return nil, err
	}
	return decompressedData, nil
}

func extractUNTSC(r io.Reader) ([]byte, error) {
	var header struct {
		Magic      [4]byte
		DecompSize uint32
		CompSize   uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if !bytes.Equal(header.Magic[:], Signatures[TypeUNTSC]) {
		return nil, errors.New("invalid UNTSC magic")
	}

	compressedData := make([]byte, header.CompSize)
	if _, err := io.ReadFull(r, compressedData); err != nil {
		return nil, err
	}

	reader := flate.NewReader(bytes.NewReader(compressedData))
	decompressedData := make([]byte, header.DecompSize)
	if _, err := io.ReadFull(reader, decompressedData); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, errors.New("decompression failed: unexpected EOF")
		}
		return nil, err
	}
	return decompressedData, nil
}

func extractUNZAR(r io.Reader) ([]byte, error) {
	magic := make([]byte, 2)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, err
	}
	if !bytes.Equal(magic, Signatures[TypeUNZAR]) {
		return nil, errors.New("invalid UNZAR magic")
	}

	var decompSize uint32
	if err := binary.Read(r, binary.LittleEndian, &decompSize); err != nil {
		return nil, err
	}

	compressedData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	reader := flate.NewReader(bytes.NewReader(compressedData))
	decompressedData := make([]byte, decompSize)
	if _, err := io.ReadFull(reader, decompressedData); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, errors.New("decompression failed: unexpected EOF")
		}
		return nil, err
	}
	return decompressedData, nil
}

func extract(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 12)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return nil, err
	}
	fileType := determineFileType(header[:n])

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	switch fileType {
	case TypeUNCMZ:
		return extractUNCMZ(f)
	case TypeUNNSK:
		return extractUNNSK(f)
	case TypeUNTSC:
		return extractUNTSC(f)
	case TypeUNZAR:
		return extractUNZAR(f)
	default:
		return nil, fmt.Errorf("unknown file type for %s", filename)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: extract <filename>")
		return
	}
	filename := os.Args[1]
	data, err := extract(filename)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	os.Stdout.Write(data)
}
