package middleware

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"lib/log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func handler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf(`{"body_length": %d}`, len(body))))
}

// 3058 ns/op
func BenchmarkWithoutRequestResponseLogger(b *testing.B) {
	body := `{"body": "abcdefghijklmnopqrstuvwxyz"}`
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

// Here we test how logger middleware decrease performance.
// Logging should not be counted.
// 5386 ns/op
// Conclusion: as logger reads body two times and writes body two times
// handler's performance is decreased by 2 000 ns.
func BenchmarkWithRequestResponseLogger(b *testing.B) {
	body := `{"body": "abcdefghijklmnopqrstuvwxyz"}`
	log.Writer = nil
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		RequestResponseLogger(handler)(w, r)
	}
}

// 22 000 ns/op
// Logging to stdout increase op time by 20 000 ns.
func BenchmarkWithRequestResponseLoggerToStdout(b *testing.B) {
	body := `{"body": "abcdefghijklmnopqrstuvwxyz"}`
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		RequestResponseLogger(handler)(w, r)
	}
}
