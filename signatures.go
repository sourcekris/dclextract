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
	TypeUNCMZ: []byte{'C', 'l', 'a', 'y'},
	TypeUNNSK: []byte{'N', 'S', 'K', 0x1A},
	TypeUNTSC: []byte{0x65, 0x5D, 0x13, 0x8C, 0x08, 0x01},
	TypeUNZAR: []byte{'P', 'T', '&'},
}

// String returns the string representation of the FileType
func (ft FileType) String() string {
	switch ft {
	case TypeUNCMZ:
		return "UNCMZ"
	case TypeUNNSK:
		return "UNNSK"
	case TypeUNTSC:
		return "UNTSC"
	case TypeUNZAR:
		return "UNZAR"
	default:
		return "Unknown"
	}
}
