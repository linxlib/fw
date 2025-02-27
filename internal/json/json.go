package json

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
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
	json.RegisterExtension(TimePlugin())
}

var (
	timeType            = reflect2.TypeOf(time.Time{})
	timeType2           = reflect2.TypeOf((*time.Time)(nil))
	timeFormatTag       = "format"
	timeLocationTag     = "location"
	defaultTimeFormat   = time.DateTime
	defaultTimeLocation = "Asia/Shanghai"
)

type timePlugin struct {
	jsoniter.DummyExtension
	timeFmtBinder Binder
}

func TimePlugin() *timePlugin {
	return &timePlugin{
		timeFmtBinder: timeFmtBinder(),
	}
}

type Binder func(*jsoniter.Binding)

func timeFmtBinder() Binder {
	return Binder(func(binding *jsoniter.Binding) {
		typ := binding.Field.Type()
		if typ == timeType {
			format, ok := binding.Field.Tag().Lookup(timeFormatTag)
			if !ok {
				format = defaultTimeFormat
			}
			location, ok := binding.Field.Tag().Lookup(timeLocationTag)
			if !ok {
				location = defaultTimeLocation
			}
			encdec := &encoderdecoder{
				encFn: timeFmtEncoder(format, location),
				decFn: timeFmtDecoder(format, location),
			}
			binding.Encoder = encdec
			binding.Decoder = encdec
		} else if typ == timeType2 {
			format, ok := binding.Field.Tag().Lookup(timeFormatTag)
			if !ok {
				format = defaultTimeFormat
			}
			location, ok := binding.Field.Tag().Lookup(timeLocationTag)
			if !ok {
				location = defaultTimeLocation
			}
			encdec := &encoderdecoder{
				encFn: timeFmtEncoder2(format, location),
				decFn: timeFmtDecoder2(format, location),
			}
			binding.Encoder = encdec
			binding.Decoder = encdec
		}
	})
}

func timeFmtEncoder2(format, location string) jsoniter.EncoderFunc {
	return jsoniter.EncoderFunc(func(ptr unsafe.Pointer, stream *jsoniter.Stream) {

		tp := (**time.Time)(ptr)
		var str string
		if *tp != nil {
			l, err := time.LoadLocation(location)
			if err != nil {
				stream.Error = err
				return
			}
			str = (*tp).In(l).Format(format)
		}
		stream.WriteString(str)
	})
}
func timeFmtEncoder(format, location string) jsoniter.EncoderFunc {
	return jsoniter.EncoderFunc(func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
		tp := (*time.Time)(ptr)
		var str string
		if tp != nil {
			l, err := time.LoadLocation(location)
			if err != nil {
				stream.Error = err
				return
			}
			str = tp.In(l).Format(format)
		}
		stream.WriteString(str)
	})
}
func timeFmtDecoder(format, location string) jsoniter.DecoderFunc {
	return jsoniter.DecoderFunc(func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
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
	})
}
func timeFmtDecoder2(format, location string) jsoniter.DecoderFunc {
	return jsoniter.DecoderFunc(func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
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
	})
}

type encoderdecoder struct {
	encFn     jsoniter.EncoderFunc
	isEmptyFn func(ptr unsafe.Pointer) bool
	decFn     jsoniter.DecoderFunc
}

func (ed *encoderdecoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	ed.decFn(ptr, iter)
}

func (ed *encoderdecoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	ed.encFn(ptr, stream)
}

func (ed *encoderdecoder) IsEmpty(ptr unsafe.Pointer) bool {
	if ed.isEmptyFn == nil {
		return false
	}
	return ed.isEmptyFn(ptr)
}

func (this *timePlugin) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		this.timeFmtBinder(binding)
	}
}
