package middleware

import (
	"bytes"
	"context"
	"fmt"
	"github.com/lithammer/shortuuid"
	"io/ioutil"
	"lib/log"
	"net/http"
	"net/http/httptest"
)

func ReferenceID(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// TODO do we trust our clients?
		ref := req.Header.Get("Reference-ID")
		if ref == "" {
			ref = shortuuid.New()
		}
		ctx := context.WithValue(req.Context(), "reference_id", ref)
		req = req.WithContext(ctx)
		w.Header().Set("Reference-ID", ref)
		handler.ServeHTTP(w, req)
	}
}

func Recover(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				refID := GetReferenceID(r)
				user := GetUser(r)

				log.Log(
					"failed handler.ServeHTTP",
					log.ReferenceID(refID),
					log.User(user),
					log.Error(fmt.Errorf("%v", err)),
					log.Context(map[string]string{"body": fmt.Sprintf("%#v", r.Body)}),
					log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, nil),
				)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf(`{"reference_id": "%s"}`, refID)))
			}
		}()

		handler.ServeHTTP(w, r)
	}
}

// It's preferable to log requests and responses of such handlers that have side effects.
// They usually (but not always) are sent as POST/PUT/DELETE requests.
// Most of handlers deal with GET requests (get details, get list, search item...).
// That is why the RequestResponseLogger is not a global middleware like the Recover.
// This middleware usually will be applied only to limited amount of handlers.
// The most flexible approach is to selectively wrap a handler func with the RequestResponseLogger.
// That is why the signature of the RequestResponseLogger differs from the signature of the Recover.
// The last one wraps a router, not a handler func.
// TODO maybe run log.Log in goroutine?
func RequestResponseLogger(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reqBody []byte
		var respBody []byte
		var err error

		refID := GetReferenceID(r)
		user := GetUser(r)

		if r.Body != nil {
			reqBody, err = ioutil.ReadAll(r.Body)
			if err != nil {
				log.Log(
					"failed ioutil.ReadAll",
					log.ReferenceID(refID),
					log.User(user),
					log.Error(err),
					log.Context(map[string]string{"body": fmt.Sprintf("%#v", r.Body)}),
					log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, nil),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf(`{"reference_id": "%s"}`, refID)))
				return
			}
			r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))
		}

		rec := httptest.NewRecorder()
		handler(rec, r)

		if rec.Body != nil {
			respBody, err = ioutil.ReadAll(rec.Body)
			if err != nil {
				log.Log(
					"failed ioutil.ReadAll",
					log.Error(err),
					log.ReferenceID(refID),
					log.User(user),
					log.Context(map[string]string{"body": fmt.Sprintf("%#v", rec.Body)}),
					log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, reqBody),
					log.Response(rec.Code, rec.Header(), nil),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf(`{"reference_id": "%s"}`, refID)))
				return
			}
		}

		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		if len(respBody) > 0 {
			_, err = w.Write(respBody)
			if err != nil {
				log.Log(
					"failed w.Write",
					log.ReferenceID(refID),
					log.User(user),
					log.Error(err),
					log.Context(map[string]string{"body": fmt.Sprintf("%#v", respBody)}),
					log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, reqBody),
					log.Response(rec.Code, rec.Header(), respBody),
				)
			}
		}

		log.Log(
			fmt.Sprintf("in '%s %s' %d", r.Method, r.Host+r.URL.Path, rec.Code),
			log.ReferenceID(refID),
			log.User(user),
			log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, reqBody),
			log.Response(rec.Code, rec.Header(), respBody),
		)
	}
}

func GetReferenceID(r *http.Request) string {
	refID := ""
	v := r.Context().Value("reference_id")
	if v != nil {
		refID = v.(string)
	}
	return refID
}

func GetUser(r *http.Request) string {
	u := ""
	v := r.Context().Value("user")
	if v != nil {
		u = v.(string)
	}
	return u
}
