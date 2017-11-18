// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_io "io"
)

var pkg_wrap_io = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"ByteReader":       reflect.ValueOf(reflect.TypeOf((*wrap_io.ByteReader)(nil)).Elem()),
		"ByteScanner":      reflect.ValueOf(reflect.TypeOf((*wrap_io.ByteScanner)(nil)).Elem()),
		"ByteWriter":       reflect.ValueOf(reflect.TypeOf((*wrap_io.ByteWriter)(nil)).Elem()),
		"Closer":           reflect.ValueOf(reflect.TypeOf((*wrap_io.Closer)(nil)).Elem()),
		"Copy":             reflect.ValueOf(wrap_io.Copy),
		"CopyBuffer":       reflect.ValueOf(wrap_io.CopyBuffer),
		"CopyN":            reflect.ValueOf(wrap_io.CopyN),
		"EOF":              reflect.ValueOf(&wrap_io.EOF).Elem(),
		"ErrClosedPipe":    reflect.ValueOf(&wrap_io.ErrClosedPipe).Elem(),
		"ErrNoProgress":    reflect.ValueOf(&wrap_io.ErrNoProgress).Elem(),
		"ErrShortBuffer":   reflect.ValueOf(&wrap_io.ErrShortBuffer).Elem(),
		"ErrShortWrite":    reflect.ValueOf(&wrap_io.ErrShortWrite).Elem(),
		"ErrUnexpectedEOF": reflect.ValueOf(&wrap_io.ErrUnexpectedEOF).Elem(),
		"LimitReader":      reflect.ValueOf(wrap_io.LimitReader),
		"LimitedReader":    reflect.ValueOf(reflect.TypeOf(wrap_io.LimitedReader{})),
		"MultiReader":      reflect.ValueOf(wrap_io.MultiReader),
		"MultiWriter":      reflect.ValueOf(wrap_io.MultiWriter),
		"NewSectionReader": reflect.ValueOf(wrap_io.NewSectionReader),
		"Pipe":             reflect.ValueOf(wrap_io.Pipe),
		"PipeReader":       reflect.ValueOf(reflect.TypeOf(wrap_io.PipeReader{})),
		"PipeWriter":       reflect.ValueOf(reflect.TypeOf(wrap_io.PipeWriter{})),
		"ReadAtLeast":      reflect.ValueOf(wrap_io.ReadAtLeast),
		"ReadCloser":       reflect.ValueOf(reflect.TypeOf((*wrap_io.ReadCloser)(nil)).Elem()),
		"ReadFull":         reflect.ValueOf(wrap_io.ReadFull),
		"ReadSeeker":       reflect.ValueOf(reflect.TypeOf((*wrap_io.ReadSeeker)(nil)).Elem()),
		"ReadWriteCloser":  reflect.ValueOf(reflect.TypeOf((*wrap_io.ReadWriteCloser)(nil)).Elem()),
		"ReadWriteSeeker":  reflect.ValueOf(reflect.TypeOf((*wrap_io.ReadWriteSeeker)(nil)).Elem()),
		"ReadWriter":       reflect.ValueOf(reflect.TypeOf((*wrap_io.ReadWriter)(nil)).Elem()),
		"Reader":           reflect.ValueOf(reflect.TypeOf((*wrap_io.Reader)(nil)).Elem()),
		"ReaderAt":         reflect.ValueOf(reflect.TypeOf((*wrap_io.ReaderAt)(nil)).Elem()),
		"ReaderFrom":       reflect.ValueOf(reflect.TypeOf((*wrap_io.ReaderFrom)(nil)).Elem()),
		"RuneReader":       reflect.ValueOf(reflect.TypeOf((*wrap_io.RuneReader)(nil)).Elem()),
		"RuneScanner":      reflect.ValueOf(reflect.TypeOf((*wrap_io.RuneScanner)(nil)).Elem()),
		"SectionReader":    reflect.ValueOf(reflect.TypeOf(wrap_io.SectionReader{})),
		"SeekCurrent":      reflect.ValueOf(wrap_io.SeekCurrent),
		"SeekEnd":          reflect.ValueOf(wrap_io.SeekEnd),
		"SeekStart":        reflect.ValueOf(wrap_io.SeekStart),
		"Seeker":           reflect.ValueOf(reflect.TypeOf((*wrap_io.Seeker)(nil)).Elem()),
		"TeeReader":        reflect.ValueOf(wrap_io.TeeReader),
		"WriteCloser":      reflect.ValueOf(reflect.TypeOf((*wrap_io.WriteCloser)(nil)).Elem()),
		"WriteSeeker":      reflect.ValueOf(reflect.TypeOf((*wrap_io.WriteSeeker)(nil)).Elem()),
		"WriteString":      reflect.ValueOf(wrap_io.WriteString),
		"Writer":           reflect.ValueOf(reflect.TypeOf((*wrap_io.Writer)(nil)).Elem()),
		"WriterAt":         reflect.ValueOf(reflect.TypeOf((*wrap_io.WriterAt)(nil)).Elem()),
		"WriterTo":         reflect.ValueOf(reflect.TypeOf((*wrap_io.WriterTo)(nil)).Elem()),
	},
}

func init() {
	if gowrap.Pkgs["io"] == nil {
		gowrap.Pkgs["io"] = pkg_wrap_io
	}
}
