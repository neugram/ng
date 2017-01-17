// Generated file, do not edit.

package gowrap

import (
	encoding_binary "encoding/binary"
	"reflect"
)

var wrap_encoding_binary = &Pkg{
	Exports: map[string]reflect.Value{

		"BigEndian":      reflect.ValueOf(encoding_binary.BigEndian),
		"ByteOrder":      reflect.ValueOf((*encoding_binary.ByteOrder)(nil)),
		"LittleEndian":   reflect.ValueOf(encoding_binary.LittleEndian),
		"MaxVarintLen16": reflect.ValueOf(encoding_binary.MaxVarintLen16),
		"MaxVarintLen32": reflect.ValueOf(encoding_binary.MaxVarintLen32),
		"MaxVarintLen64": reflect.ValueOf(encoding_binary.MaxVarintLen64),
		"PutUvarint":     reflect.ValueOf(encoding_binary.PutUvarint),
		"PutVarint":      reflect.ValueOf(encoding_binary.PutVarint),
		"Read":           reflect.ValueOf(encoding_binary.Read),
		"ReadUvarint":    reflect.ValueOf(encoding_binary.ReadUvarint),
		"ReadVarint":     reflect.ValueOf(encoding_binary.ReadVarint),
		"Size":           reflect.ValueOf(encoding_binary.Size),
		"Uvarint":        reflect.ValueOf(encoding_binary.Uvarint),
		"Varint":         reflect.ValueOf(encoding_binary.Varint),
		"Write":          reflect.ValueOf(encoding_binary.Write),
	},
}

func init() {
	Pkgs["encoding/binary"] = wrap_encoding_binary
}
