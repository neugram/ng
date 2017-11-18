// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_path "path"
)

var pkg_wrap_path = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Base":          reflect.ValueOf(wrap_path.Base),
		"Clean":         reflect.ValueOf(wrap_path.Clean),
		"Dir":           reflect.ValueOf(wrap_path.Dir),
		"ErrBadPattern": reflect.ValueOf(&wrap_path.ErrBadPattern).Elem(),
		"Ext":           reflect.ValueOf(wrap_path.Ext),
		"IsAbs":         reflect.ValueOf(wrap_path.IsAbs),
		"Join":          reflect.ValueOf(wrap_path.Join),
		"Match":         reflect.ValueOf(wrap_path.Match),
		"Split":         reflect.ValueOf(wrap_path.Split),
	},
}

func init() {
	if gowrap.Pkgs["path"] == nil {
		gowrap.Pkgs["path"] = pkg_wrap_path
	}
}
