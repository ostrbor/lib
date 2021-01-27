package main

import (
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"lib/httpclient"
	"lib/middleware"
	"net/http"
	"time"
)

func post(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	name := params.ByName("name")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "internal"}`))
		return
	}
	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "internal"}`))
		return
	}

	request, err := http.NewRequest("GET", "http://example.org", nil)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "internal"}`))
		return
	}
	resp, err := httpclient.Send(request, 5*time.Second, "")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "internal"}`))
		return
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "internal"}`))
		return
	}

	w.WriteHeader(201)
	w.Write([]byte(fmt.Sprintf(`{"result": "%s"}`, name)))
}

func get(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte(`{"version": "1"}`))
}

func getSilent(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

type x struct {
	X map[string]string `json:"x,omitempty"`
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	reqRespLog := middleware.RequestResponseLogger
	router := httprouter.New()
	router.HandlerFunc("POST", "/person/:name", reqRespLog(post))
	router.HandlerFunc("GET", "/api/v1", reqRespLog(get))
	router.HandlerFunc("GET", "/api/v1/silent", reqRespLog(getSilent))
	http.ListenAndServe(":8080", router)
}
