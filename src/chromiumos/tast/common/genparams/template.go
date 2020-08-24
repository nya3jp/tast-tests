// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package genparams

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"chromiumos/tast/errors"
)

// funcs is a map of functions available to templates.
var funcs = map[string]interface{}{
	"fmt": format,
}

// Template is a utility function to render a Go template into a string in a
// single call.
//
// Go's standard template engine is used to render a template string. See
// https://godoc.org/text/template for the template syntax.
//
// This function also installs a few helper function that can be called inside
// templates:
//
//  fmt(v interface{}) string - Formats v in a Go syntax. Supported types are
//    boolean, integer, float, string, slice of supported types, map of
//    supported types.
func Template(t TestingT, text string, data interface{}) string {
	t.Helper()

	tmpl, err := template.New("").Funcs(funcs).Parse(text)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}
	return buf.String()
}

func format(v interface{}) (string, error) {
	return formatRec(v, true)
}

func formatRec(v interface{}, needType bool) (text string, err error) {
	if v == nil {
		return "nil", nil
	}

	val := reflect.ValueOf(v)
	typ := val.Type()

	if duration, ok := v.(time.Duration); ok {
		return formatDuration(duration)
	}

	if defined(typ) {
		return "", errors.Errorf("unsupported type %T", v)
	}

	if needType {
		switch typ.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32:
			defer func() { text = fmt.Sprintf("%T(%s)", v, text) }()
		case reflect.Slice, reflect.Map:
			defer func() { text = fmt.Sprintf("%T%s", v, text) }()
		}
	}

	switch typ.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(val.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(val.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(val.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(val.Float(), 'g', -1, 64), nil
	case reflect.String:
		return strconv.Quote(val.String()), nil
	case reflect.Slice:
		var elems []string
		for i := 0; i < val.Len(); i++ {
			s, err := formatRec(val.Index(i).Interface(), false)
			if err != nil {
				return "", err
			}
			elems = append(elems, s)
		}
		return fmt.Sprintf("{%s}", strings.Join(elems, ", ")), nil
	case reflect.Map:
		keys, err := sortKeys(val.MapKeys())
		if err != nil {
			return "", err
		}
		var elems []string
		for _, key := range keys {
			sk, err := formatRec(key.Interface(), true)
			if err != nil {
				return "", err
			}
			sv, err := formatRec(val.MapIndex(key).Interface(), false)
			if err != nil {
				return "", err
			}
			elems = append(elems, fmt.Sprintf("%s: %s", sk, sv))
		}
		return fmt.Sprintf("{%s}", strings.Join(elems, ", ")), nil
	default:
		return "", errors.Errorf("unsupported type %T", v)
	}
}

func sortKeys(keys []reflect.Value) ([]reflect.Value, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	switch keys[0].Type().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Int() < keys[j].Int()
		})
		return keys, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Uint() < keys[j].Uint()
		})
		return keys, nil
	case reflect.Float32, reflect.Float64:
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].Float() < keys[j].Float()
		})
		return keys, nil
	case reflect.String:
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
		return keys, nil
	default:
		return nil, errors.Errorf("unsupported type %T for map keys", keys[0].Interface())
	}
}

// defined returns true if t is a defined type.
// https://golang.org/ref/spec#Type_definitions
func defined(t reflect.Type) bool {
	// PkgPath is empty for predeclared types and non-defined types.
	// Name is empty for non-defined types.
	return t.PkgPath() != "" && t.Name() != ""
}

func formatDuration(t time.Duration) (string, error) {
	if hours := t.Hours(); hours == math.Floor(hours) {
		return strconv.FormatInt(int64(hours), 10) + " * time.Hour", nil
	} else if minutes := t.Minutes(); minutes == math.Floor(minutes) {
		return strconv.FormatInt(int64(minutes), 10) + " * time.Minute", nil
	} else if seconds := t.Seconds(); seconds == math.Floor(seconds) {
		return strconv.FormatInt(int64(seconds), 10) + " * time.Second", nil
	} else {
		return "", errors.Errorf("time.Duration %v is too precise", t)
	}
}
