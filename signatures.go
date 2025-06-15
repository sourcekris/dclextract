package main

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
