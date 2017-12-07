// Copyright 2017 The Neugram Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jupyter

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"reflect"

	"neugram.io/ng/ngcore"
)

// genDisplayData generates a displayData message from values returned by the kernel.
func genDisplayData(s *ngcore.Session, vals []reflect.Value) (displayData, error) {
	dd := displayData{
		Data:      make(map[string]interface{}),
		Meta:      make(map[string]interface{}),
		Transient: make(map[string]interface{}),
	}

	switch len(vals) {
	case 0:
		return dd, nil
	case 1:
		// TODO: figure out what to do when there are multiple data items to display.
		// what does the reference kernel (IPython) do?
		// does it do e.g.:
		//  data["image/png"] = []string{image(0), image(1), ...}
		switch v := vals[0].Interface().(type) {
		case image.Image:
			buf := new(bytes.Buffer)
			err := png.Encode(buf, v)
			if err != nil {
				return dd, err
			}
			dd.Data["image/png"] = buf.Bytes()
		case []byte:
			mime := http.DetectContentType(v)
			dd.Data[mime] = v
		}
	}

	txt := new(bytes.Buffer)
	s.Display(txt, vals)
	dd.Data["text/plain"] = txt.String()

	return dd, nil
}
