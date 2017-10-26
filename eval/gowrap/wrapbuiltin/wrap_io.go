// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	io "io"
)

var wrap_io = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"ByteReader":       reflect.ValueOf(reflect.TypeOf((*io.ByteReader)(nil)).Elem()),
		"ByteScanner":      reflect.ValueOf(reflect.TypeOf((*io.ByteScanner)(nil)).Elem()),
		"ByteWriter":       reflect.ValueOf(reflect.TypeOf((*io.ByteWriter)(nil)).Elem()),
		"Closer":           reflect.ValueOf(reflect.TypeOf((*io.Closer)(nil)).Elem()),
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
		"LimitedReader":    reflect.ValueOf(reflect.TypeOf(io.LimitedReader{})),
		"MultiReader":      reflect.ValueOf(io.MultiReader),
		"MultiWriter":      reflect.ValueOf(io.MultiWriter),
		"NewSectionReader": reflect.ValueOf(io.NewSectionReader),
		"Pipe":             reflect.ValueOf(io.Pipe),
		"PipeReader":       reflect.ValueOf(reflect.TypeOf(io.PipeReader{})),
		"PipeWriter":       reflect.ValueOf(reflect.TypeOf(io.PipeWriter{})),
		"ReadAtLeast":      reflect.ValueOf(io.ReadAtLeast),
		"ReadCloser":       reflect.ValueOf(reflect.TypeOf((*io.ReadCloser)(nil)).Elem()),
		"ReadFull":         reflect.ValueOf(io.ReadFull),
		"ReadSeeker":       reflect.ValueOf(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()),
		"ReadWriteCloser":  reflect.ValueOf(reflect.TypeOf((*io.ReadWriteCloser)(nil)).Elem()),
		"ReadWriteSeeker":  reflect.ValueOf(reflect.TypeOf((*io.ReadWriteSeeker)(nil)).Elem()),
		"ReadWriter":       reflect.ValueOf(reflect.TypeOf((*io.ReadWriter)(nil)).Elem()),
		"Reader":           reflect.ValueOf(reflect.TypeOf((*io.Reader)(nil)).Elem()),
		"ReaderAt":         reflect.ValueOf(reflect.TypeOf((*io.ReaderAt)(nil)).Elem()),
		"ReaderFrom":       reflect.ValueOf(reflect.TypeOf((*io.ReaderFrom)(nil)).Elem()),
		"RuneReader":       reflect.ValueOf(reflect.TypeOf((*io.RuneReader)(nil)).Elem()),
		"RuneScanner":      reflect.ValueOf(reflect.TypeOf((*io.RuneScanner)(nil)).Elem()),
		"SectionReader":    reflect.ValueOf(reflect.TypeOf(io.SectionReader{})),
		"SeekCurrent":      reflect.ValueOf(io.SeekCurrent),
		"SeekEnd":          reflect.ValueOf(io.SeekEnd),
		"SeekStart":        reflect.ValueOf(io.SeekStart),
		"Seeker":           reflect.ValueOf(reflect.TypeOf((*io.Seeker)(nil)).Elem()),
		"TeeReader":        reflect.ValueOf(io.TeeReader),
		"WriteCloser":      reflect.ValueOf(reflect.TypeOf((*io.WriteCloser)(nil)).Elem()),
		"WriteSeeker":      reflect.ValueOf(reflect.TypeOf((*io.WriteSeeker)(nil)).Elem()),
		"WriteString":      reflect.ValueOf(io.WriteString),
		"Writer":           reflect.ValueOf(reflect.TypeOf((*io.Writer)(nil)).Elem()),
		"WriterAt":         reflect.ValueOf(reflect.TypeOf((*io.WriterAt)(nil)).Elem()),
		"WriterTo":         reflect.ValueOf(reflect.TypeOf((*io.WriterTo)(nil)).Elem()),
	},
}

func init() {
	gowrap.Pkgs["io"] = wrap_io
}
