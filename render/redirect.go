package render

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"net/http"
)

type Redirect struct {
	Code     int
	Location string
}

func (r Redirect) Render(w *fasthttp.RequestCtx) error {
	if (r.Code < http.StatusMultipleChoices || r.Code > http.StatusPermanentRedirect) && r.Code != http.StatusCreated {
		panic(fmt.Sprintf("Cannot redirect with status code %d", r.Code))
	}
	w.Redirect(r.Location, r.Code)
	return nil
}
func (r Redirect) WriteContentType(*fasthttp.RequestCtx) {}
