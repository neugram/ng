// Generated file, do not edit.

package wrapbuiltin

import (
	"reflect"

	"neugram.io/ng/eval/gowrap"

	wrap_time "time"
)

var pkg_wrap_time = &gowrap.Pkg{
	Exports: map[string]reflect.Value{

		"ANSIC":           reflect.ValueOf(wrap_time.ANSIC),
		"After":           reflect.ValueOf(wrap_time.After),
		"AfterFunc":       reflect.ValueOf(wrap_time.AfterFunc),
		"April":           reflect.ValueOf(wrap_time.April),
		"August":          reflect.ValueOf(wrap_time.August),
		"Date":            reflect.ValueOf(wrap_time.Date),
		"December":        reflect.ValueOf(wrap_time.December),
		"Duration":        reflect.ValueOf(reflect.TypeOf(wrap_time.Duration(0))),
		"February":        reflect.ValueOf(wrap_time.February),
		"FixedZone":       reflect.ValueOf(wrap_time.FixedZone),
		"Friday":          reflect.ValueOf(wrap_time.Friday),
		"Hour":            reflect.ValueOf(wrap_time.Hour),
		"January":         reflect.ValueOf(wrap_time.January),
		"July":            reflect.ValueOf(wrap_time.July),
		"June":            reflect.ValueOf(wrap_time.June),
		"Kitchen":         reflect.ValueOf(wrap_time.Kitchen),
		"LoadLocation":    reflect.ValueOf(wrap_time.LoadLocation),
		"Local":           reflect.ValueOf(&wrap_time.Local).Elem(),
		"Location":        reflect.ValueOf(reflect.TypeOf(wrap_time.Location{})),
		"March":           reflect.ValueOf(wrap_time.March),
		"May":             reflect.ValueOf(wrap_time.May),
		"Microsecond":     reflect.ValueOf(wrap_time.Microsecond),
		"Millisecond":     reflect.ValueOf(wrap_time.Millisecond),
		"Minute":          reflect.ValueOf(wrap_time.Minute),
		"Monday":          reflect.ValueOf(wrap_time.Monday),
		"Month":           reflect.ValueOf(reflect.TypeOf(wrap_time.Month(0))),
		"Nanosecond":      reflect.ValueOf(wrap_time.Nanosecond),
		"NewTicker":       reflect.ValueOf(wrap_time.NewTicker),
		"NewTimer":        reflect.ValueOf(wrap_time.NewTimer),
		"November":        reflect.ValueOf(wrap_time.November),
		"Now":             reflect.ValueOf(wrap_time.Now),
		"October":         reflect.ValueOf(wrap_time.October),
		"Parse":           reflect.ValueOf(wrap_time.Parse),
		"ParseDuration":   reflect.ValueOf(wrap_time.ParseDuration),
		"ParseError":      reflect.ValueOf(reflect.TypeOf(wrap_time.ParseError{})),
		"ParseInLocation": reflect.ValueOf(wrap_time.ParseInLocation),
		"RFC1123":         reflect.ValueOf(wrap_time.RFC1123),
		"RFC1123Z":        reflect.ValueOf(wrap_time.RFC1123Z),
		"RFC3339":         reflect.ValueOf(wrap_time.RFC3339),
		"RFC3339Nano":     reflect.ValueOf(wrap_time.RFC3339Nano),
		"RFC822":          reflect.ValueOf(wrap_time.RFC822),
		"RFC822Z":         reflect.ValueOf(wrap_time.RFC822Z),
		"RFC850":          reflect.ValueOf(wrap_time.RFC850),
		"RubyDate":        reflect.ValueOf(wrap_time.RubyDate),
		"Saturday":        reflect.ValueOf(wrap_time.Saturday),
		"Second":          reflect.ValueOf(wrap_time.Second),
		"September":       reflect.ValueOf(wrap_time.September),
		"Since":           reflect.ValueOf(wrap_time.Since),
		"Sleep":           reflect.ValueOf(wrap_time.Sleep),
		"Stamp":           reflect.ValueOf(wrap_time.Stamp),
		"StampMicro":      reflect.ValueOf(wrap_time.StampMicro),
		"StampMilli":      reflect.ValueOf(wrap_time.StampMilli),
		"StampNano":       reflect.ValueOf(wrap_time.StampNano),
		"Sunday":          reflect.ValueOf(wrap_time.Sunday),
		"Thursday":        reflect.ValueOf(wrap_time.Thursday),
		"Tick":            reflect.ValueOf(wrap_time.Tick),
		"Ticker":          reflect.ValueOf(reflect.TypeOf(wrap_time.Ticker{})),
		"Time":            reflect.ValueOf(reflect.TypeOf(wrap_time.Time{})),
		"Timer":           reflect.ValueOf(reflect.TypeOf(wrap_time.Timer{})),
		"Tuesday":         reflect.ValueOf(wrap_time.Tuesday),
		"UTC":             reflect.ValueOf(&wrap_time.UTC).Elem(),
		"Unix":            reflect.ValueOf(wrap_time.Unix),
		"UnixDate":        reflect.ValueOf(wrap_time.UnixDate),
		"Until":           reflect.ValueOf(wrap_time.Until),
		"Wednesday":       reflect.ValueOf(wrap_time.Wednesday),
		"Weekday":         reflect.ValueOf(reflect.TypeOf(wrap_time.Weekday(0))),
	},
}

func init() {
	if gowrap.Pkgs["time"] == nil {
		gowrap.Pkgs["time"] = pkg_wrap_time
	}
}
