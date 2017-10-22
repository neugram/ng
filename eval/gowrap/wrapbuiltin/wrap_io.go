// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	io "io"
)

var wrap_io = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"ByteReader":       reflect.ValueOf((*io.ByteReader)(nil)),
		"ByteScanner":      reflect.ValueOf((*io.ByteScanner)(nil)),
		"ByteWriter":       reflect.ValueOf((*io.ByteWriter)(nil)),
		"Closer":           reflect.ValueOf((*io.Closer)(nil)),
		"Copy":             reflect.ValueOf(io.Copy),
		"CopyBuffer":       reflect.ValueOf(io.CopyBuffer),
		"CopyN":            reflect.ValueOf(io.CopyN),
		"EOF":              reflect.ValueOf(io.EOF),
		"ErrClosedPipe":    reflect.ValueOf(io.ErrClosedPipe),
		"ErrNoProgress":    reflect.ValueOf(io.ErrNoProgress),
		"ErrShortBuffer":   reflect.ValueOf(io.ErrShortBuffer),
		"ErrShortWrite":    reflect.ValueOf(io.ErrShortWrite),
		"ErrUnexpectedEOF": reflect.ValueOf(io.ErrUnexpectedEOF),
		"LimitReader":      reflect.ValueOf(io.LimitReader),
		"LimitedReader":    reflect.ValueOf(io.LimitedReader{}),
		"MultiReader":      reflect.ValueOf(io.MultiReader),
		"MultiWriter":      reflect.ValueOf(io.MultiWriter),
		"NewSectionReader": reflect.ValueOf(io.NewSectionReader),
		"Pipe":             reflect.ValueOf(io.Pipe),
		"PipeReader":       reflect.ValueOf(io.PipeReader{}),
		"PipeWriter":       reflect.ValueOf(io.PipeWriter{}),
		"ReadAtLeast":      reflect.ValueOf(io.ReadAtLeast),
		"ReadCloser":       reflect.ValueOf((*io.ReadCloser)(nil)),
		"ReadFull":         reflect.ValueOf(io.ReadFull),
		"ReadSeeker":       reflect.ValueOf((*io.ReadSeeker)(nil)),
		"ReadWriteCloser":  reflect.ValueOf((*io.ReadWriteCloser)(nil)),
		"ReadWriteSeeker":  reflect.ValueOf((*io.ReadWriteSeeker)(nil)),
		"ReadWriter":       reflect.ValueOf((*io.ReadWriter)(nil)),
		"Reader":           reflect.ValueOf((*io.Reader)(nil)),
		"ReaderAt":         reflect.ValueOf((*io.ReaderAt)(nil)),
		"ReaderFrom":       reflect.ValueOf((*io.ReaderFrom)(nil)),
		"RuneReader":       reflect.ValueOf((*io.RuneReader)(nil)),
		"RuneScanner":      reflect.ValueOf((*io.RuneScanner)(nil)),
		"SectionReader":    reflect.ValueOf(io.SectionReader{}),
		"SeekCurrent":      reflect.ValueOf(io.SeekCurrent),
		"SeekEnd":          reflect.ValueOf(io.SeekEnd),
		"SeekStart":        reflect.ValueOf(io.SeekStart),
		"Seeker":           reflect.ValueOf((*io.Seeker)(nil)),
		"TeeReader":        reflect.ValueOf(io.TeeReader),
		"WriteCloser":      reflect.ValueOf((*io.WriteCloser)(nil)),
		"WriteSeeker":      reflect.ValueOf((*io.WriteSeeker)(nil)),
		"WriteString":      reflect.ValueOf(io.WriteString),
		"Writer":           reflect.ValueOf((*io.Writer)(nil)),
		"WriterAt":         reflect.ValueOf((*io.WriterAt)(nil)),
		"WriterTo":         reflect.ValueOf((*io.WriterTo)(nil)),
	},
}

func init() {
	gowrap.Pkgs["io"] = wrap_io
}
