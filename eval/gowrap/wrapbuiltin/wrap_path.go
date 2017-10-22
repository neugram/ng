// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	path "path"
)

var wrap_path = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"Base":          reflect.ValueOf(path.Base),
		"Clean":         reflect.ValueOf(path.Clean),
		"Dir":           reflect.ValueOf(path.Dir),
		"ErrBadPattern": reflect.ValueOf(path.ErrBadPattern),
		"Ext":           reflect.ValueOf(path.Ext),
		"IsAbs":         reflect.ValueOf(path.IsAbs),
		"Join":          reflect.ValueOf(path.Join),
		"Match":         reflect.ValueOf(path.Match),
		"Split":         reflect.ValueOf(path.Split),
	},
}

func init() {
	gowrap.Pkgs["path"] = wrap_path
}
