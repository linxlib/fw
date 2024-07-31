package fw

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"github.com/google/uuid"
	"github.com/gookit/color"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw/internal/json"
	"github.com/sirupsen/logrus"
	"mime"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

type H map[string]any

func (h H) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{
		Space: "",
		Local: "map",
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for key, value := range h {
		elem := xml.StartElement{
			Name: xml.Name{Space: "", Local: key},
			Attr: []xml.Attr{},
		}
		if err := e.EncodeElement(value, elem); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func joinRoute(base string, r string) string {
	var result = base
	if r == "/" || r == "" {

		if strings.HasSuffix(result, "/") {
			result = strings.TrimSuffix(result, "/")
			r = ""
		} else {
			r = strings.TrimSuffix(r, "/")
			result += r
		}
	} else {
		if strings.HasSuffix(result, "/") {
			r = strings.TrimPrefix(r, "/")
			result += r
		} else {
			r = strings.TrimPrefix(r, "/")
			result += "/" + r
		}
	}
	return result
}
func Stringify(v interface{}) string {
	var ret string
	if IsObject(v) || isMap(v) {
		ret = marshal(v)
	} else {
		ret = fmt.Sprint(v)
	}
	return ret
}
func IsObject(v interface{}) bool {
	return reflect.ValueOf(v).Kind() == reflect.Struct
}
func marshal(o interface{}) string {
	str, ok := o.(string)
	if ok {
		return str
	}
	data, err := json.Marshal(o)
	if err != nil {
		return fmt.Sprint(o)
	}
	return string(data)
}
func unmarshal(data string, o interface{}) error {
	return json.Unmarshal(conv.Bytes(data), o)
}
func marshalIndent(o interface{}) string {
	str, ok := o.(string)
	if ok {
		return str
	}
	m, err := json.MarshalIndent(o, "", " ")
	if err != nil {
		return fmt.Sprint(o)
	}
	return string(m)
}
func isMap(v interface{}) bool {
	_, isMap := v.(map[string]interface{})
	_, isLogFields := v.(logrus.Fields)

	return isMap || isLogFields
}

var (
	white        = color.HiWhite.Render
	red          = color.HiRed.Render
	green        = color.HiGreen.Render
	blue         = color.HiBlue.Render
	darkBlue     = color.Blue.Render
	cyan         = color.HiCyan.Render
	gray         = color.Gray.Render
	yellow       = color.HiYellow.Render
	magenta      = color.HiMagenta.Render
	lightmagenta = color.LightMagenta.Render
	info         = color.Info.Render
	note         = color.Note.Render
	err          = color.Error.Render
	danger       = color.Danger.Render
	success      = color.Success.Render
	warning      = color.Warn.Render
	question     = color.Question.Render
	primary      = color.Primary.Render
	secondary    = color.Secondary.Render
)

type byteSeq interface {
	~string | ~[]byte
}

const (
	toLowerTable = "\x00\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-./0123456789:;<=>?@abcdefghijklmnopqrstuvwxyz[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~\u007f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"
	toUpperTable = "\x00\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`ABCDEFGHIJKLMNOPQRSTUVWXYZ{|}~\u007f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"
)

// EqualFold tests ascii strings or bytes for equality case-insensitively
func EqualFold[S byteSeq](b, s S) bool {
	if len(b) != len(s) {
		return false
	}
	for i := len(b) - 1; i >= 0; i-- {
		if toUpperTable[b[i]] != toUpperTable[s[i]] {
			return false
		}
	}
	return true
}

// TrimLeft is the equivalent of strings/bytes.TrimLeft
func TrimLeft[S byteSeq](s S, cutset byte) S {
	lenStr, start := len(s), 0
	for start < lenStr && s[start] == cutset {
		start++
	}
	return s[start:]
}

// Trim is the equivalent of strings/bytes.Trim
func Trim[S byteSeq](s S, cutset byte) S {
	i, j := 0, len(s)-1
	for ; i <= j; i++ {
		if s[i] != cutset {
			break
		}
	}
	for ; i < j; j-- {
		if s[j] != cutset {
			break
		}
	}

	return s[i : j+1]
}

// TrimRight is the equivalent of strings/bytes.TrimRight
func TrimRight[S byteSeq](s S, cutset byte) S {
	lenStr := len(s)
	for lenStr > 0 && s[lenStr-1] == cutset {
		lenStr--
	}
	return s[:lenStr]
}

// Copyright © 2014, Roger Peppe
// github.com/rogpeppe/fastuuid
// All rights reserved.

var (
	uuidSeed    [24]byte
	uuidCounter uint64
	uuidSetup   sync.Once
	unitsSlice  = []byte("kmgtp")
)

// UUID generates an universally unique identifier (UUID)
func UUID() string {
	// Setup seed & counter once
	uuidSetup.Do(func() {
		if _, err := rand.Read(uuidSeed[:]); err != nil {
			return
		}
		uuidCounter = binary.LittleEndian.Uint64(uuidSeed[:8])
	})
	if atomic.LoadUint64(&uuidCounter) <= 0 {
		return "00000000-0000-0000-0000-000000000000"
	}
	// first 8 bytes differ, taking a slice of the first 16 bytes
	x := atomic.AddUint64(&uuidCounter, 1)
	_uuid := uuidSeed
	binary.LittleEndian.PutUint64(_uuid[:8], x)
	_uuid[6], _uuid[9] = _uuid[9], _uuid[6]

	// RFC4122 v4
	_uuid[6] = (_uuid[6] & 0x0f) | 0x40
	_uuid[8] = _uuid[8]&0x3f | 0x80

	// create UUID representation of the first 128 bits
	b := make([]byte, 36)
	hex.Encode(b[0:8], _uuid[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], _uuid[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], _uuid[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], _uuid[8:10])
	b[23] = '-'
	hex.Encode(b[24:], _uuid[10:16])

	return conv.String(b)
}

// UUIDv4 returns a Random (Version 4) UUID.
// The strength of the UUIDs is based on the strength of the crypto/rand package.
func UUIDv4() string {
	token, err := uuid.NewRandom()
	if err != nil {
		return UUID()
	}
	return token.String()
}

type GenericType interface {
	GenericTypeInteger | GenericTypeFloat | bool | string | []byte
}

type GenericTypeInteger interface {
	GenericTypeIntegerSigned | GenericTypeIntegerUnsigned
}

type GenericTypeIntegerSigned interface {
	int | int8 | int16 | int32 | int64
}

type GenericTypeIntegerUnsigned interface {
	uint | uint8 | uint16 | uint32 | uint64
}

type GenericTypeFloat interface {
	float32 | float64
}

const MIMEOctetStream = "application/octet-stream"

// GetMIME returns the content-type of a file extension
func GetMIME(extension string) string {
	if len(extension) == 0 {
		return ""
	}
	var foundMime string
	if extension[0] == '.' {
		foundMime = mimeExtensions[extension[1:]]
	} else {
		foundMime = mimeExtensions[extension]
	}

	if len(foundMime) == 0 {
		if extension[0] != '.' {
			foundMime = mime.TypeByExtension("." + extension)
		} else {
			foundMime = mime.TypeByExtension(extension)
		}

		if foundMime == "" {
			return MIMEOctetStream
		}
	}
	return foundMime
}

// MIME types were copied from https://github.com/nginx/nginx/blob/67d2a9541826ecd5db97d604f23460210fd3e517/conf/mime.types with the following updates:
// - Use "application/xml" instead of "text/xml" as recommended per https://datatracker.ietf.org/doc/html/rfc7303#section-4.1
// - Use "text/javascript" instead of "application/javascript" as recommended per https://www.rfc-editor.org/rfc/rfc9239#name-text-javascript
var mimeExtensions = map[string]string{
	"html":    "text/html",
	"htm":     "text/html",
	"shtml":   "text/html",
	"css":     "text/css",
	"xml":     "application/xml",
	"gif":     "image/gif",
	"jpeg":    "image/jpeg",
	"jpg":     "image/jpeg",
	"js":      "text/javascript",
	"atom":    "application/atom+xml",
	"rss":     "application/rss+xml",
	"mml":     "text/mathml",
	"txt":     "text/plain",
	"jad":     "text/vnd.sun.j2me.app-descriptor",
	"wml":     "text/vnd.wap.wml",
	"htc":     "text/x-component",
	"avif":    "image/avif",
	"png":     "image/png",
	"svg":     "image/svg+xml",
	"svgz":    "image/svg+xml",
	"tif":     "image/tiff",
	"tiff":    "image/tiff",
	"wbmp":    "image/vnd.wap.wbmp",
	"webp":    "image/webp",
	"ico":     "image/x-icon",
	"jng":     "image/x-jng",
	"bmp":     "image/x-ms-bmp",
	"woff":    "font/woff",
	"woff2":   "font/woff2",
	"jar":     "application/java-archive",
	"war":     "application/java-archive",
	"ear":     "application/java-archive",
	"json":    "application/json",
	"hqx":     "application/mac-binhex40",
	"doc":     "application/msword",
	"pdf":     "application/pdf",
	"ps":      "application/postscript",
	"eps":     "application/postscript",
	"ai":      "application/postscript",
	"rtf":     "application/rtf",
	"m3u8":    "application/vnd.apple.mpegurl",
	"kml":     "application/vnd.google-earth.kml+xml",
	"kmz":     "application/vnd.google-earth.kmz",
	"xls":     "application/vnd.ms-excel",
	"eot":     "application/vnd.ms-fontobject",
	"ppt":     "application/vnd.ms-powerpoint",
	"odg":     "application/vnd.oasis.opendocument.graphics",
	"odp":     "application/vnd.oasis.opendocument.presentation",
	"ods":     "application/vnd.oasis.opendocument.spreadsheet",
	"odt":     "application/vnd.oasis.opendocument.text",
	"pptx":    "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"xlsx":    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"docx":    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"wmlc":    "application/vnd.wap.wmlc",
	"wasm":    "application/wasm",
	"7z":      "application/x-7z-compressed",
	"cco":     "application/x-cocoa",
	"jardiff": "application/x-java-archive-diff",
	"jnlp":    "application/x-java-jnlp-file",
	"run":     "application/x-makeself",
	"pl":      "application/x-perl",
	"pm":      "application/x-perl",
	"prc":     "application/x-pilot",
	"pdb":     "application/x-pilot",
	"rar":     "application/x-rar-compressed",
	"rpm":     "application/x-redhat-package-manager",
	"sea":     "application/x-sea",
	"swf":     "application/x-shockwave-flash",
	"sit":     "application/x-stuffit",
	"tcl":     "application/x-tcl",
	"tk":      "application/x-tcl",
	"der":     "application/x-x509-ca-cert",
	"pem":     "application/x-x509-ca-cert",
	"crt":     "application/x-x509-ca-cert",
	"xpi":     "application/x-xpinstall",
	"xhtml":   "application/xhtml+xml",
	"xspf":    "application/xspf+xml",
	"zip":     "application/zip",
	"zst":     "application/zstd",
	"bin":     "application/octet-stream",
	"exe":     "application/octet-stream",
	"dll":     "application/octet-stream",
	"deb":     "application/octet-stream",
	"dmg":     "application/octet-stream",
	"iso":     "application/octet-stream",
	"img":     "application/octet-stream",
	"msi":     "application/octet-stream",
	"msp":     "application/octet-stream",
	"msm":     "application/octet-stream",
	"mid":     "audio/midi",
	"midi":    "audio/midi",
	"kar":     "audio/midi",
	"mp3":     "audio/mpeg",
	"ogg":     "audio/ogg",
	"m4a":     "audio/x-m4a",
	"ra":      "audio/x-realaudio",
	"3gpp":    "video/3gpp",
	"3gp":     "video/3gpp",
	"ts":      "video/mp2t",
	"mp4":     "video/mp4",
	"mpeg":    "video/mpeg",
	"mpg":     "video/mpeg",
	"mov":     "video/quicktime",
	"webm":    "video/webm",
	"flv":     "video/x-flv",
	"m4v":     "video/x-m4v",
	"mng":     "video/x-mng",
	"asx":     "video/x-ms-asf",
	"asf":     "video/x-ms-asf",
	"wmv":     "video/x-ms-wmv",
	"avi":     "video/x-msvideo",
}

// assertValueType asserts the type of the result to the type of the value
func assertValueType[V GenericType, T any](result T) V {
	v, ok := any(result).(V)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to %T", v))
	}
	return v
}

func genericParseDefault[V GenericType](err error, parser func() V, defaultValue ...V) V {
	var v V
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return v
	}
	return parser()
}

func genericParseInt[V GenericType](str string, bitSize int, parser func(int64) V, defaultValue ...V) V {
	result, err := strconv.ParseInt(str, 10, bitSize)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseUint[V GenericType](str string, bitSize int, parser func(uint64) V, defaultValue ...V) V {
	result, err := strconv.ParseUint(str, 10, bitSize)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseFloat[V GenericType](str string, bitSize int, parser func(float64) V, defaultValue ...V) V {
	result, err := strconv.ParseFloat(str, bitSize)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseBool[V GenericType](str string, parser func(bool) V, defaultValue ...V) V {
	result, err := strconv.ParseBool(str)
	return genericParseDefault[V](err, func() V { return parser(result) }, defaultValue...)
}

func genericParseType[V GenericType](str string, v V, defaultValue ...V) V {
	switch any(v).(type) {
	case int:
		return genericParseInt[V](str, 0, func(i int64) V { return assertValueType[V, int](int(i)) }, defaultValue...)
	case int8:
		return genericParseInt[V](str, 8, func(i int64) V { return assertValueType[V, int8](int8(i)) }, defaultValue...)
	case int16:
		return genericParseInt[V](str, 16, func(i int64) V { return assertValueType[V, int16](int16(i)) }, defaultValue...)
	case int32:
		return genericParseInt[V](str, 32, func(i int64) V { return assertValueType[V, int32](int32(i)) }, defaultValue...)
	case int64:
		return genericParseInt[V](str, 64, func(i int64) V { return assertValueType[V, int64](i) }, defaultValue...)
	case uint:
		return genericParseUint[V](str, 32, func(i uint64) V { return assertValueType[V, uint](uint(i)) }, defaultValue...)
	case uint8:
		return genericParseUint[V](str, 8, func(i uint64) V { return assertValueType[V, uint8](uint8(i)) }, defaultValue...)
	case uint16:
		return genericParseUint[V](str, 16, func(i uint64) V { return assertValueType[V, uint16](uint16(i)) }, defaultValue...)
	case uint32:
		return genericParseUint[V](str, 32, func(i uint64) V { return assertValueType[V, uint32](uint32(i)) }, defaultValue...)
	case uint64:
		return genericParseUint[V](str, 64, func(i uint64) V { return assertValueType[V, uint64](i) }, defaultValue...)
	case float32:
		return genericParseFloat[V](str, 32, func(i float64) V { return assertValueType[V, float32](float32(i)) }, defaultValue...)
	case float64:
		return genericParseFloat[V](str, 64, func(i float64) V { return assertValueType[V, float64](i) }, defaultValue...)
	case bool:
		return genericParseBool[V](str, func(b bool) V { return assertValueType[V, bool](b) }, defaultValue...)
	case string:
		if str == "" && len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return assertValueType[V, string](str)
	case []byte:
		if str == "" && len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return assertValueType[V, []byte]([]byte(str))
	default:
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return v
	}
}

// limits for HTTP statuscodes
const (
	statusMessageMin = 100
	statusMessageMax = 511
)

// StatusMessage returns the correct message for the provided HTTP statuscode
func StatusMessage(status int) string {
	if status < statusMessageMin || status > statusMessageMax {
		return ""
	}
	return statusMessage[status]
}

// NOTE: Keep this in sync with fiber's status code list
var statusMessage = []string{
	100: "Continue",            // StatusContinue
	101: "Switching Protocols", // StatusSwitchingProtocols
	102: "Processing",          // StatusProcessing
	103: "Early Hints",         // StatusEarlyHints

	200: "OK",                            // StatusOK
	201: "Created",                       // StatusCreated
	202: "Accepted",                      // StatusAccepted
	203: "Non-Authoritative Information", // StatusNonAuthoritativeInformation
	204: "No Content",                    // StatusNoContent
	205: "Reset Content",                 // StatusResetContent
	206: "Partial Content",               // StatusPartialContent
	207: "Multi-Status",                  // StatusMultiStatus
	208: "Already Reported",              // StatusAlreadyReported
	226: "IM Used",                       // StatusIMUsed

	300: "Multiple Choices",   // StatusMultipleChoices
	301: "Moved Permanently",  // StatusMovedPermanently
	302: "Found",              // StatusFound
	303: "See Other",          // StatusSeeOther
	304: "Not Modified",       // StatusNotModified
	305: "Use Proxy",          // StatusUseProxy
	306: "Switch Proxy",       // StatusSwitchProxy
	307: "Temporary Redirect", // StatusTemporaryRedirect
	308: "Permanent Redirect", // StatusPermanentRedirect

	400: "Bad Request",                     // StatusBadRequest
	401: "Unauthorized",                    // StatusUnauthorized
	402: "Payment Required",                // StatusPaymentRequired
	403: "Forbidden",                       // StatusForbidden
	404: "Not Found",                       // StatusNotFound
	405: "Method Not Allowed",              // StatusMethodNotAllowed
	406: "Not Acceptable",                  // StatusNotAcceptable
	407: "Proxy Authentication Required",   // StatusProxyAuthRequired
	408: "Request Timeout",                 // StatusRequestTimeout
	409: "Conflict",                        // StatusConflict
	410: "Gone",                            // StatusGone
	411: "Length Required",                 // StatusLengthRequired
	412: "Precondition Failed",             // StatusPreconditionFailed
	413: "Request Entity Too Large",        // StatusRequestEntityTooLarge
	414: "Request URI Too Long",            // StatusRequestURITooLong
	415: "Unsupported Media Type",          // StatusUnsupportedMediaType
	416: "Requested Range Not Satisfiable", // StatusRequestedRangeNotSatisfiable
	417: "Expectation Failed",              // StatusExpectationFailed
	418: "I'm a teapot",                    // StatusTeapot
	421: "Misdirected Request",             // StatusMisdirectedRequest
	422: "Unprocessable Entity",            // StatusUnprocessableEntity
	423: "Locked",                          // StatusLocked
	424: "Failed Dependency",               // StatusFailedDependency
	425: "Too Early",                       // StatusTooEarly
	426: "Upgrade Required",                // StatusUpgradeRequired
	428: "Precondition Required",           // StatusPreconditionRequired
	429: "Too Many Requests",               // StatusTooManyRequests
	431: "Request Header Fields Too Large", // StatusRequestHeaderFieldsTooLarge
	451: "Unavailable For Legal Reasons",   // StatusUnavailableForLegalReasons

	500: "Internal Server Error",           // StatusInternalServerError
	501: "Not Implemented",                 // StatusNotImplemented
	502: "Bad Gateway",                     // StatusBadGateway
	503: "Service Unavailable",             // StatusServiceUnavailable
	504: "Gateway Timeout",                 // StatusGatewayTimeout
	505: "HTTP Version Not Supported",      // StatusHTTPVersionNotSupported
	506: "Variant Also Negotiates",         // StatusVariantAlsoNegotiates
	507: "Insufficient Storage",            // StatusInsufficientStorage
	508: "Loop Detected",                   // StatusLoopDetected
	510: "Not Extended",                    // StatusNotExtended
	511: "Network Authentication Required", // StatusNetworkAuthenticationRequired
}
