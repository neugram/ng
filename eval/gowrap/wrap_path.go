// Generated file, do not edit.

package gowrap

import (
	path "path"
	"reflect"
)

var wrap_path = &Pkg{
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
	Pkgs["path"] = wrap_path
}
