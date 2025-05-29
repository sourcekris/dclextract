package main

// FileType represents a type of compressed file
type FileType int

const (
	// TypeUNCMZ represents a UNCMZ compressed file
	TypeUNCMZ FileType = iota
	// TypeUNNSK represents a UNNSK compressed file
	TypeUNNSK
	// TypeUNTSC represents a UNTSC compressed file
	TypeUNTSC
	// TypeUNZAR represents a UNZAR compressed file
	TypeUNZAR
	// TypeUnknown represents an unknown file type
	TypeUnknown
)

// Signatures holds the magic byte signatures for each file type
var Signatures = map[FileType][]byte{
	TypeUNCMZ: []byte{'C', 'M', 'P', 'Z'},
	TypeUNNSK: []byte{'N', 'S', 'K', 0x1A},
	TypeUNTSC: []byte{'T', 'S', 'C', 0x1A},
	TypeUNZAR: []byte{0x1F, 0x9E},
}
