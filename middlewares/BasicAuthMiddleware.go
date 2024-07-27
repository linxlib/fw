package middlewares

import (
	"crypto/subtle"
	"encoding/base64"
	"github.com/linxlib/fw"
	"github.com/linxlib/fw/internal/bytesconv"
	"net/http"
	"net/url"
	"strconv"
)

var _ fw.IMiddlewareCtl = (*BasicAuthMiddleware)(nil)

const (
	AuthUserKey      = "user"
	AuthProxyUserKey = "proxy_user"
)

type Accounts map[string]string
type authPair struct {
	user  string
	value string
}
type authPairs []authPair

func (a authPairs) searchCredential(authValue string) (string, bool) {
	if authValue == "" {
		return "", false
	}
	for _, pair := range a {
		if subtle.ConstantTimeCompare(bytesconv.StringToBytes(pair.value), bytesconv.StringToBytes(authValue)) == 1 {
			return pair.user, true
		}
	}
	return "", false
}
func processAccounts(accounts Accounts) authPairs {
	length := len(accounts)
	if length <= 0 {
		panic("Empty list of authorized credentials")
	}
	pairs := make(authPairs, 0, length)
	for user, password := range accounts {
		if user == "" {
			panic("User can not be empty")
		}
		value := authorizationHeader(user, password)
		pairs = append(pairs, authPair{
			value: value,
			user:  user,
		})
	}
	return pairs
}
func authorizationHeader(user, password string) string {
	base := user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString(bytesconv.StringToBytes(base))
}

type BasicAuthMiddleware struct {
	fw.MiddlewareCtl
	realm string
	proxy bool
	pairs authPairs
}

func (b *BasicAuthMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return b.CloneAsCtl()
}

func (b *BasicAuthMiddleware) HandlerMethod(h fw.HandlerFunc) fw.HandlerFunc {
	var p = b.GetParam()
	values, err := url.ParseQuery(p)
	if err != nil {
		panic("BasicAuthMiddleware: param for this middleware is invalid, should like `@BasicAuth proxy=true&realm=xxx&user1=pass1&user2=pass2`")
	} else {
		if v := values.Get("proxy"); v != "" {
			b.proxy = v == "true"
		}
		if v := values.Get("realm"); v != "" {
			b.realm = v
		}
		if b.realm == "" {
			if b.proxy {
				b.realm = "Proxy Authorization Required"
			} else {
				b.realm = "Authorization Required"
			}
		}
		b.realm = "Basic realm=" + strconv.Quote(b.realm)
		values.Del("realm")
		t := map[string][]string(values)
		accounts := make(Accounts)
		for s, strings := range t {
			accounts[s] = strings[0]
		}
		b.pairs = processAccounts(accounts)

		if b.proxy {
			return func(context *fw.Context) {
				bs := context.GetFastContext().Request.Header.Peek("Proxy-Authorization")
				proxyUser, found := b.pairs.searchCredential(string(bs))
				if !found {
					// Credentials doesn't match, we return 407 and abort handlers chain.
					context.GetFastContext().Response.Header.Set("Proxy-Authenticate", b.realm)
					context.GetFastContext().Response.SetStatusCode(http.StatusProxyAuthRequired)
					return
				}
				context.Set(AuthProxyUserKey, proxyUser)
				h(context)
			}
		} else { //basic auth
			return func(context *fw.Context) {
				bs := context.GetFastContext().Request.Header.Peek("Authorization")
				user, found := b.pairs.searchCredential(string(bs))
				if !found {
					// Credentials doesn't match, we return 407 and abort handlers chain.
					context.GetFastContext().Response.Header.Set("WWW-Authenticate", b.realm)
					context.GetFastContext().Response.SetStatusCode(http.StatusUnauthorized)
					return
				}
				context.Set(AuthUserKey, user)
				h(context)
			}
		}

	}

}

func (b *BasicAuthMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewBasicAuthMiddleware()
}

func (b *BasicAuthMiddleware) HandlerController(base string) *fw.RouteItem {
	return &fw.RouteItem{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: b,
	}
}

func NewBasicAuthMiddleware() fw.IMiddlewareCtl {
	return &BasicAuthMiddleware{
		MiddlewareCtl: fw.NewMiddlewareCtl("BasicAuth", "BasicAuth"),
	}
}
