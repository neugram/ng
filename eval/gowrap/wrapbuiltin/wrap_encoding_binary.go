// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_encoding_binary "encoding/binary"
)

var pkg_wrap_encoding_binary = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"BigEndian":      reflect.ValueOf(wrap_encoding_binary.BigEndian),
		"ByteOrder":      reflect.ValueOf(reflect.TypeOf((*wrap_encoding_binary.ByteOrder)(nil)).Elem()),
		"LittleEndian":   reflect.ValueOf(wrap_encoding_binary.LittleEndian),
		"MaxVarintLen16": reflect.ValueOf(wrap_encoding_binary.MaxVarintLen16),
		"MaxVarintLen32": reflect.ValueOf(wrap_encoding_binary.MaxVarintLen32),
		"MaxVarintLen64": reflect.ValueOf(wrap_encoding_binary.MaxVarintLen64),
		"PutUvarint":     reflect.ValueOf(wrap_encoding_binary.PutUvarint),
		"PutVarint":      reflect.ValueOf(wrap_encoding_binary.PutVarint),
		"Read":           reflect.ValueOf(wrap_encoding_binary.Read),
		"ReadUvarint":    reflect.ValueOf(wrap_encoding_binary.ReadUvarint),
		"ReadVarint":     reflect.ValueOf(wrap_encoding_binary.ReadVarint),
		"Size":           reflect.ValueOf(wrap_encoding_binary.Size),
		"Uvarint":        reflect.ValueOf(wrap_encoding_binary.Uvarint),
		"Varint":         reflect.ValueOf(wrap_encoding_binary.Varint),
		"Write":          reflect.ValueOf(wrap_encoding_binary.Write),
	},
}

func init() {
	if gowrap.Pkgs["encoding/binary"] == nil {
		gowrap.Pkgs["encoding/binary"] = pkg_wrap_encoding_binary
	}
}
