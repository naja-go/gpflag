package gpflag

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Codecs own parsing configuration, never a bound variable or occurrence state.
// Implementations are immutable so inherited registrations can share them.
type codec[T any] interface {
	parse(current T, text string, first bool) (T, error)
	format(T) string
}

type codecOption[T any] struct {
	codec     codec[T]
	name      string
	noOpt     string
	parseOnly bool
}

func (o codecOption[T]) apply(f *Flag[T]) {
	next := o.codec
	if o.parseOnly {
		formatting := f.codec
		if previous, ok := formatting.(parseOverrideCodec[T]); ok {
			formatting = previous.formatting
		}
		next = parseOverrideCodec[T]{parsing: next, formatting: formatting}
	}
	f.codec, f.inner.native = next, false
	if o.name != "" {
		f.inner.Type = o.name
	}
	if o.noOpt != "" {
		f.inner.NoOptDefVal = o.noOpt
	}
}

// Parsing-only options retain a previously selected formatter. Neither codec
// owns a flag's value or occurrence state.
type parseOverrideCodec[T any] struct {
	parsing    codec[T]
	formatting codec[T]
}

func (c parseOverrideCodec[T]) parse(current T, text string, first bool) (T, error) {
	return c.parsing.parse(current, text, first)
}
func (c parseOverrideCodec[T]) format(value T) string { return c.formatting.format(value) }

// WithParser supplies typed parsing and formatting. first is true before the
// first successful assignment. Do not mutate shared input data before returning
// an error: custom pointer unmarshaling cannot be rolled back by the package.
func WithParser[T any](parse func(T, string, bool) (T, error), format func(T) string) Option[T] {
	if parse == nil || format == nil {
		panic("gpflag: nil parser or formatter")
	}
	return codecOption[T]{codec: callbackCodec[T]{parse, format}}
}

// Function fields are reserved for callbacks explicitly supplied by the caller.
type callbackCodec[T any] struct {
	parseFunc  func(T, string, bool) (T, error)
	formatFunc func(T) string
}

func (c callbackCodec[T]) parse(current T, text string, first bool) (T, error) {
	return c.parseFunc(current, text, first)
}
func (c callbackCodec[T]) format(value T) string { return c.formatFunc(value) }

// Count increments an int on each bare occurrence. Explicit values replace it.
func Count() Option[int] {
	return codecOption[int]{codec: countCodec{}, name: "count", noOpt: "+1", parseOnly: true}
}

// StringArray appends each argument verbatim rather than splitting on commas.
func StringArray() Option[[]string] {
	return codecOption[[]string]{codec: stringArrayCodec{}, name: "stringArray", parseOnly: true}
}

// Base64 selects base64 rather than hexadecimal encoding for []byte.
func Base64() Option[[]byte] {
	return codecOption[[]byte]{codec: bytesCodec{base64: true}, name: "bytesBase64"}
}

// TimeFormats selects accepted time layouts without changing the formatter.
// The default formatter is RFC3339Nano.
func TimeFormats(layouts ...string) Option[time.Time] {
	if len(layouts) == 0 {
		panic("gpflag: TimeFormats requires at least one layout")
	}
	return codecOption[time.Time]{codec: timeCodec{layouts: append([]string(nil), layouts...)}, parseOnly: true}
}

func setCodec[T any](f *Flag[T], name string, c codec[T], native bool) {
	f.codec, f.inner.Type, f.inner.native = c, name, native
}

func configureParser[T any](flag *Flag[T]) {
	switch f := any(flag).(type) {
	case *Flag[string]:
		setCodec(f, "string", stringCodec{}, true)
	case *Flag[bool]:
		setCodec(f, "bool", boolCodec{}, true)
		f.inner.boolFlag, f.inner.NoOptDefVal = true, "true"
	case *Flag[int]:
		setCodec(f, "int", signedCodec[int]{}, true)
	case *Flag[int8]:
		setCodec(f, "int8", signedCodec[int8]{}, true)
	case *Flag[int16]:
		setCodec(f, "int16", signedCodec[int16]{}, true)
	case *Flag[int32]:
		setCodec(f, "int32", signedCodec[int32]{}, true)
	case *Flag[int64]:
		setCodec(f, "int64", signedCodec[int64]{}, true)
	case *Flag[uint]:
		setCodec(f, "uint", unsignedCodec[uint]{}, true)
	case *Flag[uint8]:
		setCodec(f, "uint8", unsignedCodec[uint8]{}, true)
	case *Flag[uint16]:
		setCodec(f, "uint16", unsignedCodec[uint16]{}, true)
	case *Flag[uint32]:
		setCodec(f, "uint32", unsignedCodec[uint32]{}, true)
	case *Flag[uint64]:
		setCodec(f, "uint64", unsignedCodec[uint64]{}, true)
	case *Flag[float32]:
		setCodec(f, "float32", floatCodec[float32]{}, true)
	case *Flag[float64]:
		setCodec(f, "float64", floatCodec[float64]{}, true)
	case *Flag[time.Duration]:
		setCodec(f, "duration", durationCodec{}, true)
	case *Flag[time.Time]:
		setCodec(f, "time", timeCodec{layouts: []string{time.RFC3339Nano}}, false)
	case *Flag[[]string]:
		setCodec(f, "stringSlice", sliceCodec[string]{element: stringCodec{}, syntax: csvList}, false)
	case *Flag[[]bool]:
		setCodec(f, "boolSlice", sliceCodec[bool]{element: boolCodec{}, syntax: looseList}, false)
	case *Flag[[]int]:
		setCodec(f, "intSlice", sliceCodec[int]{element: signedCodec[int]{base: 10}}, false)
	case *Flag[[]int32]:
		setCodec(f, "int32Slice", sliceCodec[int32]{element: signedCodec[int32]{}}, false)
	case *Flag[[]int64]:
		setCodec(f, "int64Slice", sliceCodec[int64]{element: signedCodec[int64]{}}, false)
	case *Flag[[]uint]:
		setCodec(f, "uintSlice", sliceCodec[uint]{element: unsignedCodec[uint]{}}, false)
	case *Flag[[]float32]:
		setCodec(f, "float32Slice", sliceCodec[float32]{element: floatCodec[float32]{}}, false)
	case *Flag[[]float64]:
		setCodec(f, "float64Slice", sliceCodec[float64]{element: floatCodec[float64]{}}, false)
	case *Flag[[]time.Duration]:
		setCodec(f, "durationSlice", sliceCodec[time.Duration]{element: durationCodec{}}, false)
	case *Flag[[]byte]:
		setCodec(f, "bytesHex", bytesCodec{}, false)
	case *Flag[net.IP]:
		setCodec(f, "ip", ipCodec{allowEmpty: true}, false)
	case *Flag[net.IPMask]:
		setCodec(f, "ipMask", maskCodec{}, false)
	case *Flag[net.IPNet]:
		setCodec(f, "ipNet", ipNetCodec{}, false)
	case *Flag[[]net.IP]:
		setCodec(f, "ipSlice", sliceCodec[net.IP]{element: ipCodec{}, syntax: looseList}, false)
	case *Flag[[]net.IPNet]:
		setCodec(f, "ipNetSlice", sliceCodec[net.IPNet]{element: ipNetCodec{}, syntax: looseList}, false)
	case *Flag[map[string]string]:
		setCodec(f, "stringToString", mapCodec[string]{element: stringCodec{}, quoted: true}, false)
	case *Flag[map[string]int]:
		setCodec(f, "stringToInt", mapCodec[int]{element: signedCodec[int]{base: 10}}, false)
	case *Flag[map[string]int64]:
		setCodec(f, "stringToInt64", mapCodec[int64]{element: signedCodec[int64]{base: 10}}, false)
	default:
		_, pointer := any(new(T)).(encoding.TextUnmarshaler)
		_, value := any(*new(T)).(encoding.TextUnmarshaler)
		if pointer || value {
			flag.codec = textCodec[T]{}
			if reflect.TypeFor[T]().Kind() == reflect.Bool {
				flag.inner.boolFlag, flag.inner.NoOptDefVal = true, "true"
			}
		}
	}
}

type stringCodec struct{}

func (stringCodec) parse(_ string, text string, _ bool) (string, error) { return text, nil }
func (stringCodec) format(value string) string                          { return value }

type boolCodec struct{}

func (boolCodec) parse(_ bool, text string, _ bool) (bool, error) {
	value, err := strconv.ParseBool(text)
	if err != nil {
		return false, errors.New("must be true or false")
	}
	return value, nil
}
func (boolCodec) format(value bool) string { return strconv.FormatBool(value) }

type signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}
type unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}
type floating interface{ ~float32 | ~float64 }

type signedCodec[T signed] struct{ base int }

func (c signedCodec[T]) parse(_ T, text string, _ bool) (T, error) {
	value, err := strconv.ParseInt(text, c.base, int(reflect.TypeFor[T]().Size())*8)
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	return T(value), nil
}
func (signedCodec[T]) format(value T) string { return strconv.FormatInt(int64(value), 10) }

type unsignedCodec[T unsigned] struct{}

func (unsignedCodec[T]) parse(_ T, text string, _ bool) (T, error) {
	value, err := strconv.ParseUint(text, 0, int(reflect.TypeFor[T]().Size())*8)
	if err != nil {
		return 0, errors.New("must be a non-negative integer")
	}
	return T(value), nil
}
func (unsignedCodec[T]) format(value T) string { return strconv.FormatUint(uint64(value), 10) }

type floatCodec[T floating] struct{}

func (floatCodec[T]) parse(_ T, text string, _ bool) (T, error) {
	value, err := strconv.ParseFloat(text, int(reflect.TypeFor[T]().Size())*8)
	if err != nil {
		return 0, errors.New("must be a number")
	}
	return T(value), nil
}
func (floatCodec[T]) format(value T) string {
	return strconv.FormatFloat(float64(value), 'g', -1, int(reflect.TypeFor[T]().Size())*8)
}

type countCodec struct{}

func (countCodec) parse(current int, text string, _ bool) (int, error) {
	if text == "+1" {
		return current + 1, nil
	}
	return strconv.Atoi(text)
}
func (countCodec) format(value int) string { return strconv.Itoa(value) }

type durationCodec struct{}

func (durationCodec) parse(_ time.Duration, text string, _ bool) (time.Duration, error) {
	value, err := time.ParseDuration(text)
	if err != nil {
		return 0, errors.New(`must be a duration like "30s" or "5m"`)
	}
	return value, nil
}
func (durationCodec) format(value time.Duration) string { return value.String() }

type timeCodec struct{ layouts []string }

func (c timeCodec) parse(_ time.Time, text string, _ bool) (time.Time, error) {
	text = strings.TrimSpace(text)
	for _, layout := range c.layouts {
		if value, err := time.Parse(layout, text); err == nil {
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time format `%s` must be one of: `%s`", text, strings.Join(c.layouts, "`, `"))
}
func (timeCodec) format(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

type bytesCodec struct{ base64 bool }

func (c bytesCodec) parse(_ []byte, text string, _ bool) ([]byte, error) {
	text = strings.TrimSpace(text)
	if c.base64 {
		return base64.StdEncoding.DecodeString(text)
	}
	return hex.DecodeString(text)
}
func (c bytesCodec) format(value []byte) string {
	if c.base64 {
		return base64.StdEncoding.EncodeToString(value)
	}
	return strings.ToUpper(hex.EncodeToString(value))
}

type ipCodec struct{ allowEmpty bool }

func (c ipCodec) parse(current net.IP, text string, _ bool) (net.IP, error) {
	if text == "" && c.allowEmpty {
		return current, nil
	}
	ip := net.ParseIP(strings.TrimSpace(text))
	if ip == nil {
		return nil, fmt.Errorf("failed to parse IP: %q", text)
	}
	return ip, nil
}
func (ipCodec) format(value net.IP) string { return value.String() }

type ipNetCodec struct{}

func (ipNetCodec) parse(_ net.IPNet, text string, _ bool) (net.IPNet, error) {
	_, network, err := net.ParseCIDR(strings.TrimSpace(text))
	if err != nil {
		return net.IPNet{}, fmt.Errorf("invalid string being converted to CIDR: %s", text)
	}
	return *network, nil
}
func (ipNetCodec) format(value net.IPNet) string { return value.String() }

type maskCodec struct{}

func (maskCodec) parse(_ net.IPMask, text string, _ bool) (net.IPMask, error) {
	mask := ParseIPv4Mask(text)
	if mask == nil {
		return nil, fmt.Errorf("failed to parse IP mask: %q", text)
	}
	return mask, nil
}
func (maskCodec) format(value net.IPMask) string { return value.String() }

type textCodec[T any] struct{}

func (textCodec[T]) parse(current T, text string, _ bool) (T, error) {
	if isNilPointer(current) {
		return current, fmt.Errorf("cannot unmarshal text into nil %s", reflect.TypeFor[T]())
	}
	next := current
	var err error
	if u, ok := any(&next).(encoding.TextUnmarshaler); ok {
		err = u.UnmarshalText([]byte(text))
	} else {
		err = any(next).(encoding.TextUnmarshaler).UnmarshalText([]byte(text))
	}
	return next, err
}
func (textCodec[T]) format(value T) string {
	if isNilPointer(value) {
		return "<nil>"
	}
	if m, ok := any(value).(encoding.TextMarshaler); ok {
		if text, err := m.MarshalText(); err == nil {
			return string(text)
		}
	}
	if m, ok := any(&value).(encoding.TextMarshaler); ok {
		if text, err := m.MarshalText(); err == nil {
			return string(text)
		}
	}
	return fmt.Sprint(value)
}

type listSyntax uint8

const (
	commaList listSyntax = iota
	csvList
	looseList
)

func (s listSyntax) split(text string) ([]string, error) {
	switch s {
	case csvList:
		return readAsCSV(text)
	case looseList:
		items, err := readAsCSV(strings.NewReplacer(`"`, "", "'", "", "`", "").Replace(text))
		for i := range items {
			items[i] = strings.TrimSpace(items[i])
		}
		return items, err
	default:
		return strings.Split(text, ","), nil
	}
}

type sliceCodec[T any] struct {
	element codec[T]
	syntax  listSyntax
}

func (c sliceCodec[T]) parse(current []T, text string, first bool) ([]T, error) {
	items, err := c.syntax.split(text)
	if err != nil {
		return nil, err
	}
	next := make([]T, len(items))
	for i, item := range items {
		next[i], err = c.element.parse(next[i], item, true)
		if err != nil {
			return nil, err
		}
	}
	if first {
		return next, nil
	}
	return append(current, next...), nil
}
func (c sliceCodec[T]) format(value []T) string {
	items := make([]string, len(value))
	for i, item := range value {
		items[i] = c.element.format(item)
	}
	text, _ := writeAsCSV(items)
	return "[" + text + "]"
}

type stringArrayCodec struct{}

func (stringArrayCodec) parse(current []string, text string, first bool) ([]string, error) {
	if first {
		return []string{text}, nil
	}
	return append(current, text), nil
}
func (stringArrayCodec) format(value []string) string {
	text, _ := writeAsCSV(value)
	return "[" + text + "]"
}

type mapCodec[T any] struct {
	element codec[T]
	quoted  bool
}

func (c mapCodec[T]) parse(current map[string]T, text string, first bool) (map[string]T, error) {
	var pairs []string
	var err error
	if c.quoted {
		switch strings.Count(text, "=") {
		case 0:
			return nil, fmt.Errorf("%s must be formatted as key=value", text)
		case 1:
			pairs = []string{strings.Trim(text, `"`)}
		default:
			pairs, err = readAsCSV(text)
		}
	} else {
		pairs = strings.Split(text, ",")
	}
	if err != nil {
		return nil, err
	}
	next := make(map[string]T, len(pairs))
	for _, pair := range pairs {
		key, text, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("%s must be formatted as key=value", pair)
		}
		var zero T
		value, err := c.element.parse(zero, text, true)
		if err != nil {
			return nil, err
		}
		next[key] = value
	}
	if current == nil {
		current = make(map[string]T)
	}
	if first {
		clear(current)
	}
	maps.Copy(current, next)
	return current, nil
}
func (c mapCodec[T]) format(value map[string]T) string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, key+"="+c.element.format(value[key]))
	}
	text, _ := writeAsCSV(items)
	return "[" + text + "]"
}

func readAsCSV(text string) ([]string, error) {
	if text == "" {
		return []string{}, nil
	}
	r := csv.NewReader(strings.NewReader(text))
	items, err := r.Read()
	if err != nil {
		return nil, err
	}
	if _, err = r.Read(); err != io.EOF {
		return nil, fmt.Errorf("must be a single CSV record; quote fields containing newlines")
	}
	return items, nil
}
func writeAsCSV(items []string) (string, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(items); err != nil {
		return "", err
	}
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n"), w.Error()
}
