package gw

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// A fake for the exodus-gw service.
type fakeGw struct {
	t *testing.T

	// List of IDs of publish objects to be created, or empty if creating publishes
	// should fail
	createPublishIds []string

	// Existing publish objects
	publishes publishMap

	// If non-nil, forces next HTTP request to return this error.
	nextHTTPError error

	// If non-nil, forces next HTTP request to return this response
	nextHTTPResponse *http.Response
}

type publishMap map[string]*fakePublish

type fakePublish struct {
	id    string
	items []ItemInput

	// If publish is committed, then each time the task state is polled,
	// we'll pop the next state from here.
	taskStates []string
}

func (p *fakePublish) nextState() string {
	out := p.taskStates[0]
	p.taskStates = p.taskStates[1:]
	return out
}

// Implement RoundTripper interface for fake handling of HTTP requests.
func (f *fakeGw) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil {
		defer r.Body.Close()
	}

	if f.nextHTTPError != nil {
		err := f.nextHTTPError
		f.nextHTTPError = nil
		return nil, err
	}

	if f.nextHTTPResponse != nil {
		out := f.nextHTTPResponse
		f.nextHTTPResponse = nil
		return out, nil
	}

	out := &http.Response{}

	out.Status = "404 Not Found"
	out.StatusCode = 404

	path := strings.TrimPrefix(r.URL.Path, "/")
	route := strings.Split(path, "/")

	if len(route) == 2 && route[0] == "task" && r.Method == "GET" {
		return f.getTask(route[1]), nil
	}

	// For every other route, path must be under /env/ suffix, bail out
	// early if not
	if route[0] != "env" {
		f.t.Logf("unexpected request path %v", path)
		return out, nil
	}
	route = route[1:]

	if len(route) == 1 && route[0] == "publish" && r.Method == "POST" {
		return f.createPublish(), nil
	}

	if len(route) == 2 && route[0] == "publish" && r.Method == "PUT" {
		return f.addPublishItems(r, route[1]), nil
	}

	if len(route) == 3 && route[0] == "publish" && route[2] == "commit" && r.Method == "POST" {
		return f.commitPublish(route[1]), nil
	}

	return out, nil
}

func newFakeGw(t *testing.T, c *client) *fakeGw {
	out := &fakeGw{t: t, createPublishIds: make([]string, 0), publishes: make(publishMap)}
	out.install(c)
	return out
}

func (f *fakeGw) install(c *client) {
	c.httpClient.Transport = f
}

func (f *fakeGw) createPublish() *http.Response {
	out := &http.Response{}

	if len(f.createPublishIds) == 0 {
		f.t.Log("out of publish IDs!")
		out.Status = "500 Internal Server Error"
		out.StatusCode = 500
		return out
	}

	id := f.createPublishIds[0]
	f.createPublishIds = f.createPublishIds[1:]

	newPublish := &fakePublish{}
	newPublish.id = id
	// By default, any new publish will successfully commit.
	newPublish.taskStates = []string{"NOT_STARTED", "IN_PROGRESS", "COMPLETE"}

	f.publishes[id] = newPublish

	content := fmt.Sprintf(`{
		"id": "%s",
		"env": "env",
		"state": "PENDING",
		"links": {
			"self": "/env/publish/%[1]s",
			"commit": "/env/publish/%[1]s/commit"
		},
		"items": []
	}`, id)
	out.Body = io.NopCloser(strings.NewReader(content))

	out.Header = http.Header{}
	out.Header.Add("Content-Type", "application/json")
	out.Status = "200 OK"
	out.StatusCode = 200

	return out
}

func (f *fakeGw) addPublishItems(r *http.Request, id string) *http.Response {
	out := &http.Response{}

	publish, havePublish := f.publishes[id]
	if !havePublish {
		f.t.Logf("requested nonexistent publish %s", id)
		out.Status = "404 Not Found"
		out.StatusCode = 404
		return out
	}

	dec := json.NewDecoder(r.Body)
	requestItems := make([]map[string]string, 0)

	err := dec.Decode(&requestItems)
	if err != nil {
		f.t.Logf("non-JSON request body? err = %v", err)
		out.Status = "400 Bad Request"
		out.StatusCode = 400
		return out
	}

	for _, item := range requestItems {
		publish.items = append(publish.items, ItemInput{item["web_uri"], item["object_key"], item["link_to"]})
	}

	out.Status = "200 OK"
	out.StatusCode = 200
	out.Body = io.NopCloser(strings.NewReader("{}"))
	return out
}

func (f *fakeGw) commitPublish(id string) *http.Response {
	out := &http.Response{}

	publish, havePublish := f.publishes[id]
	if !havePublish {
		f.t.Logf("requested nonexistent publish %s", id)
		out.Status = "404 Not Found"
		out.StatusCode = 404
		return out
	}

	if len(publish.taskStates) == 0 {
		// test can set taskStates empty to force an error
		out.Status = "500 Internal Server Error"
		out.StatusCode = 500
		return out
	}

	state := publish.nextState()
	taskID := "task-" + publish.id
	content := fmt.Sprintf(`{
		"id": "%s",
		"publish_id": "%s",
		"state": "%s",
		"links": {
			"self": "/task/%[1]s"
		}
	}`, taskID, id, state)

	out.Status = "200 OK"
	out.StatusCode = 200
	out.Body = io.NopCloser(strings.NewReader(content))
	return out
}

func (f *fakeGw) getTask(id string) *http.Response {
	out := &http.Response{}

	if !strings.HasPrefix(id, "task-") {
		f.t.Logf("requested non-task ID %s", id)
		out.Status = "404 Not Found"
		out.StatusCode = 404
		return out
	}

	publishID := strings.TrimPrefix(id, "task-")
	publish := f.publishes[publishID]
	if len(publish.taskStates) == 0 {
		out.Status = "500 Internal Server Error"
		out.StatusCode = 500
		return out
	}

	state := publish.nextState()
	content := fmt.Sprintf(`{
		"id": "%s",
		"publish_id": "%s",
		"state": "%s",
		"links": {
			"self": "/task/%[1]s"
		}
	}`, id, publishID, state)

	out.Status = "200 OK"
	out.StatusCode = 200
	out.Body = io.NopCloser(strings.NewReader(content))
	return out
}
