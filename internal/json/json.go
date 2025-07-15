package json

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var (
	// Marshal is exported by gin/json package.
	Marshal = json.Marshal
	// Unmarshal is exported by gin/json package.
	Unmarshal = json.Unmarshal
	// MarshalIndent is exported by gin/json package.
	MarshalIndent = json.MarshalIndent
	// NewDecoder is exported by gin/json package.
	NewDecoder = json.NewDecoder
	// NewEncoder is exported by gin/json package.
	NewEncoder = json.NewEncoder
)

func init() {
	json.RegisterExtension(&timePlugin{
		timeFmtBinder: timeFmtBinder(),
	})
}

var (
	timeType          = reflect2.TypeOf(time.Time{})
	timeType2         = reflect2.TypeOf((*time.Time)(nil))
	timeFormatTag     = "time_format"
	timeLocationTag   = "time_location"
	timeUTC           = "time_utc"
	defaultTimeFormat = time.DateTime
)

type timePlugin struct {
	jsoniter.DummyExtension
	timeFmtBinder Binder
}

type Binder func(*jsoniter.Binding)

func timeFmtBinder() Binder {
	return func(binding *jsoniter.Binding) {
		typ := binding.Field.Type()
		if typ == timeType {
			format, ok := binding.Field.Tag().Lookup(timeFormatTag)
			if !ok {
				format = defaultTimeFormat
			}

			l := time.Local.String()
			if isUTC, _ := strconv.ParseBool(binding.Field.Tag().Get(timeUTC)); isUTC {
				l = time.UTC.String()
			}
			location, ok := binding.Field.Tag().Lookup(timeLocationTag)
			if ok {
				l = location
			}
			codec := &encoderAndDecoder{
				encFn: timeFmtEncoder(format, l),
				decFn: timeFmtDecoder(format, l),
			}

			binding.Encoder = codec
			binding.Decoder = codec
		} else if typ == timeType2 {
			format, ok := binding.Field.Tag().Lookup(timeFormatTag)
			if !ok {
				format = defaultTimeFormat
			}
			l := time.Local.String()
			if isUTC, _ := strconv.ParseBool(binding.Field.Tag().Get(timeUTC)); isUTC {
				l = time.UTC.String()
			}
			location, ok := binding.Field.Tag().Lookup(timeLocationTag)
			if ok {
				l = location
			}
			codec := &encoderAndDecoder{
				encFn: timeFmtEncoder2(format, l),
				decFn: timeFmtDecoder2(format, l),
			}

			binding.Encoder = codec
			binding.Decoder = codec
		}
	}
}

func timeFmtEncoder2(format, location string) jsoniter.EncoderFunc {
	return func(ptr unsafe.Pointer, stream *jsoniter.Stream) {

		tp := (**time.Time)(ptr)
		if *tp != nil {
			l, err := time.LoadLocation(location)
			if err != nil {
				stream.Error = err
				return
			}
			switch tf := strings.ToLower(format); tf {
			case "unix":
				stream.WriteInt64((*tp).In(l).Unix())
				return
			case "unixnano":
				stream.WriteInt64((*tp).In(l).UnixNano())
				return
			default:
				stream.WriteString((*tp).In(l).Format(format))
				return
			}

		}
		stream.WriteString("")
	}
}
func timeFmtEncoder(format, location string) jsoniter.EncoderFunc {
	return func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
		tp := (*time.Time)(ptr)
		if tp != nil {
			l, err := time.LoadLocation(location)
			if err != nil {
				stream.Error = err
				return
			}
			switch tf := strings.ToLower(format); tf {
			case "unix":
				stream.WriteInt64((*tp).In(l).Unix())
				return
			case "unixnano":
				stream.WriteInt64((*tp).In(l).UnixNano())
				return
			default:
				stream.WriteString((*tp).In(l).Format(format))
				return
			}
		}
		stream.WriteString("")
	}
}
func timeFmtDecoder(format, location string) jsoniter.DecoderFunc {
	return func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
		switch tf := strings.ToLower(format); tf {
		case "unix", "unixnano":
			tv := iter.ReadInt64()
			d := time.Duration(1)
			if tf == "unixnano" {
				d = time.Second
			}
			t := time.Unix(tv/int64(d), tv%int64(d))
			tp := (*time.Time)(ptr)
			*tp = t
			return
		default:
			str := iter.ReadString()
			var (
				l   *time.Location
				t   time.Time
				err error
			)
			if str != "" {
				l, err = time.LoadLocation(location)
				if err != nil {
					iter.Error = err
					return
				}
				t, err = time.ParseInLocation(format, str, l)
				if err != nil {
					iter.Error = err
					return
				}
			}
			tp := (*time.Time)(ptr)
			*tp = t
		}

	}
}
func timeFmtDecoder2(format, location string) jsoniter.DecoderFunc {
	return func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {

		switch tf := strings.ToLower(format); tf {
		case "unix", "unixnano":
			tv := iter.ReadInt64()
			d := time.Duration(1)
			if tf == "unixnano" {
				d = time.Second
			}
			t := time.Unix(tv/int64(d), tv%int64(d))
			tp := (**time.Time)(ptr)
			*tp = &t
			**tp = t
			return

		default:
			str := iter.ReadString()
			var (
				l   *time.Location
				t   time.Time
				err error
			)
			if str != "" {
				l, err = time.LoadLocation(location)
				if err != nil {
					iter.Error = err
					return
				}
				t, err = time.ParseInLocation(format, str, l)
				if err != nil {
					iter.Error = err
					return
				}
			}
			tp := (**time.Time)(ptr)
			*tp = &t
			**tp = t
		}

	}
}

type encoderAndDecoder struct {
	encFn     jsoniter.EncoderFunc
	isEmptyFn func(ptr unsafe.Pointer) bool
	decFn     jsoniter.DecoderFunc
	isUnix    bool
}

func (ed *encoderAndDecoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	ed.decFn(ptr, iter)
}

func (ed *encoderAndDecoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	ed.encFn(ptr, stream)
}

func (ed *encoderAndDecoder) IsEmpty(ptr unsafe.Pointer) bool {
	if ed.isEmptyFn == nil {
		return false
	}
	return ed.isEmptyFn(ptr)
}

func (tp *timePlugin) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		tp.timeFmtBinder(binding)
	}
}
