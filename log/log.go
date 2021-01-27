// For the most small and medium projects logging of events in JSON format is very simple.
// Libraries that provide it are too complicated for such an easy task.
// This package differs from them and has several advantages:
//  - simplicity
//  - brevity
//  - uniform log structure
// Disadvantages:
//  - performance
//  - does not handle high load of logging (sampling, throttling...)
//
// Notes on performance:
// Result of benchmark test for logging to stdout without context:
//   BenchmarkLog-8   	    122086	        8721 ns/op
// Logging in zap takes 900 ns/op (10 times faster).
// 10 000 ns == 0.01 of millisecond
// Each log in handler will increase latency by 0.01 millisecond.
// As we can see performance is not a big deal for most projects.
// If RPS = 10 000, then node will spend 100 millisecond out of each second of CPU on logging.
//
// Notes on high load of logging:
// If total RPS is 10 000 and all handlers send log for each request,
// then net throughput will be ~ 10Mb/sec (1 Kb of log message).
// Mongodb can handle more than 10 000 inserts per second.
// Therefore, high load of logging is not a problem for most projects.
// Notification spam can be solved by using cache of messages and errors in notifiers.
package log

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Body with size over BodyLimit will not be logged.
const BodyLimit = 2 * 1 << 10

// HOSTNAME is set only in bash and is not present in environment variables (check by env command).
// That's why os.Getenv("HOSTNAME") returns empty string.
var HOSTNAME, _ = os.Hostname()

// TODO stack trace
type event struct {
	// Short description of event. Details are provided by other fields.
	// Examples:
	//
	//  1. Error is returned by a function call.
	//
	//     "failed db.CreateArticle"
	//
	//     Use 'failed', followed by a name of a function call.
	//     No need to use preposition 'to'. No need to use synonyms for a function name.
	//
	//  2. Job feedback.
	//     It's important to give feedback on job completion. Because otherwise it's not clear if job was done at all.
	//
	//     "succeeded track-job"
	//
	//     Use 'succeeded', followed by a name of a job.
	//
	//  3. HTTP transaction (transaction is request and response to this request).
	//     In case of failed transaction, client may reference it in order to understand what went wrong.
	//     In order to solve such issues it's important to log request and response.
	//     Especially when request handling run such operations as:
	//       - insert/update/delete record in database
	//       - validations/calculations
	//     To make logs more readable it's better to have both request and response in one logs message.
	//     Handlers can not only receive requests, they often send them. That's why it's better to
	//     clarify direction of transaction.
	//
	//     "in 'POST localhost:8080/api/v1/article' 201"
	//     "out 'GET example.org' 200"
	//
	//     Use 'in' and 'out' to indicate direction of transaction.
	//       in - handler received this request and responds to it with this response.
	//       out - handler sent this request and in return got this response.
	//     Do not write path query, as it might increase message length and clutter it with unnecessary details.
	Message string `json:"message"`

	// Should be set by logs sender, not by logs receiver.
	// Format: RFC3339, UTC timezone.
	Timestamp string `json:"timestamp"`

	// Use cases:
	//  - to track connected chain of events,
	//  - user can reference specific request, as reference id is supposed to be sent to user in case of errors.
	ReferenceID string `json:"reference_id,omitempty"`

	// Most of the requests are not made anonymously.
	// To improve readability of request and speed of debug,
	// because there will be no need to identify user of request by e.g. token in headers or request body.
	User string `json:"user,omitempty"`

	// Short name of the error, without details.
	// Used by notifiers: if error exists then notifier will send notification.
	Error string `json:"error,omitempty"`

	// String representation of function arguments, constants...
	// Type of value should be identified from the code where event happened.
	// No need to save numbers as integers:
	//   - log database is unlikely to be used to calculate values.
	// No need to serialize structures as JSON:
	//   - many structures are not intended to be represented as JSON,
	//   - it's unlikely to query log database by structure field.
	// For structure serializing prefer "%#v". NOTE: values of reference type are not readable!
	Context map[string]string `json:"context,omitempty"`

	Request  *request  `json:"request,omitempty"`
	Response *response `json:"response,omitempty"`

	// In kubernetes HOSTNAME is equal to pod's name.
	// In docker-compose use 'hostname' param to set HOSTNAME  inside container.
	Hostname string `json:"hostname,omitempty"`
}

// Customize Writer for project in init function.
// For example:
//   log.Writer = ioutil.Discard
// NOTE: Writer must be concurrently safe.
var Writer io.Writer = stdout{}

func Log(message string, setters ...SetFieldValue) {
	if Writer == nil {
		return
	}
	e := event{
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Hostname:  HOSTNAME,
	}
	for _, set := range setters {
		set(&e)
	}
	// If there is a need to improve performance, create encoder for event structure.
	// It's possible to avoid using reflection in encoder because we know types of each value
	// in event structure in advance.
	log, err := json.Marshal(&e)
	if err != nil {
		log = []byte(fmt.Sprintf(`{"message": "failed json.Marshal", "error": %q, "reference_id": %q, "context": {"event": "%#v"}}`, err, e.ReferenceID, e))
	}
	if _, err = Writer.Write(log); err != nil {
		fmt.Printf(`{"message": "failed l.Writer.Write", "error": %q, "reference_id": %q, "context": {"data": %q}}`, err, e.ReferenceID, string(log))
	}
}

type SetFieldValue func(*event)

func ReferenceID(referenceID string) SetFieldValue {
	return func(e *event) {
		e.ReferenceID = referenceID
	}
}

func Error(err error) SetFieldValue {
	return func(e *event) {
		e.Error = err.Error()
	}
}

func User(user string) SetFieldValue {
	return func(e *event) {
		e.User = user
	}
}

func Context(cnt map[string]string) SetFieldValue {
	return func(e *event) {
		// json.Marshal(nil) == "null"
		// "context" field is expected to be JSON object, so we skip initializing it with string "null".
		if cnt == nil {
			return
		}
		e.Context = cnt
	}
}

type request struct {
	Method string     `json:"method,omitempty"`
	Host   string     `json:"host,omitempty"`
	Path   string     `json:"path,omitempty"`
	Query  url.Values `json:"query,omitempty"`

	// Multiple header values are joined by comma.
	Headers map[string]string `json:"headers,omitempty"`

	Body string `json:"body,omitempty"`
}

func Request(method, host, path string, query url.Values, headers http.Header, body []byte) SetFieldValue {
	return func(e *event) {
		// To avoid logging of empty object, it's value must be nil.
		if method == "" && host == "" && path == "" && len(query) == 0 && len(headers) == 0 && len(body) == 0 {
			return
		}
		e.Request = &request{
			Method:  method,
			Host:    host,
			Path:    path,
			Query:   query,
			Headers: formatHeaders(headers),
			Body:    formatBody(body),
		}
	}
}

type response struct {
	StatusCode int `json:"status_code"`

	// Multiple header values are joined by comma.
	Headers map[string]string `json:"headers,omitempty"`

	Body string `json:"body,omitempty"`
}

func Response(statusCode int, headers http.Header, body []byte) SetFieldValue {
	return func(e *event) {
		// To avoid logging of empty object, it's value must be nil.
		if statusCode == 0 && len(headers) == 0 && len(body) == 0 {
			return
		}
		e.Response = &response{
			StatusCode: statusCode,
			Headers:    formatHeaders(headers),
			Body:       formatBody(body),
		}
	}
}

func formatHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	h := make(map[string]string)
	for header, values := range headers {
		h[header] = strings.Join(values, ", ")
	}
	return h
}

// TODO what if body has newlines, how it will be saved in mongodb?
func formatBody(body []byte) string {
	b := ""
	if len(body) == 0 {
		return b
	}
	if len(body) > BodyLimit {
		return fmt.Sprintf("not logged: body size (%d bytes) is bigger than limit (%d bytes)", len(body), BodyLimit)
	}
	return string(body)
}

type stdout struct{}

// Newline is appended, otherwise all logs will be written as one line.
func (w stdout) Write(p []byte) (n int, err error) {
	n, err = os.Stdout.Write(p)
	if err != nil {
		return
	}
	os.Stdout.Write([]byte("\n"))
	return
}
