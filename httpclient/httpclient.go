package httpclient

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"lib/log"
	"net/http"
	"time"
)

func Send(r *http.Request, timeout time.Duration, referenceID string) (*http.Response, error) {
	var reqBody []byte
	var respBody []byte
	var err error

	if r.Body != nil {
		reqBody, err = ioutil.ReadAll(r.Body)
		if err != nil {
			log.Log(
				"failed ioutil.ReadAll",
				log.ReferenceID(referenceID),
				log.Error(err),
				log.Context(map[string]string{"body": fmt.Sprintf("%#v", r.Body)}),
				log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, nil),
			)
			return nil, err
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))
	}

	client := http.Client{Timeout: timeout}
	resp, err := client.Do(r)
	if err != nil {
		log.Log(
			"failed client.Do",
			log.ReferenceID(referenceID),
			log.Error(err),
			log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, reqBody),
		)
		return nil, err
	}

	if resp.Body != nil {
		respBody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Log(
				"failed ioutil.ReadAll",
				log.ReferenceID(referenceID),
				log.Error(err),
				log.Context(map[string]string{"body": fmt.Sprintf("%#v", resp.Body)}),
				log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, reqBody),
				log.Response(resp.StatusCode, resp.Header, nil),
			)
			return nil, err
		}
		// TODO should we close body?
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(respBody))
	}

	log.Log(
		fmt.Sprintf("out '%s %s' %d", r.Method, r.Host+r.URL.Path, resp.StatusCode),
		log.ReferenceID(referenceID),
		log.Request(r.Method, r.Host, r.URL.Path, r.URL.Query(), r.Header, reqBody),
		log.Response(resp.StatusCode, resp.Header, respBody),
	)
	return resp, nil
}
