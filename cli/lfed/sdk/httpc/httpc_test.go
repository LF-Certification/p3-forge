package httpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusMapping(t *testing.T) {
	cases := []struct {
		status   int
		body     string
		sentinel error
		want     string
	}{
		{401, `{"detail":"missing username claim"}`, ErrUnauthorized, `unauthorized; run "lfed auth login": missing username claim`},
		{403, `{"detail":"nope"}`, ErrForbidden, "forbidden: nope"},
		{404, `{"detail":"hosting target not found"}`, ErrNotFound, "not found: hosting target not found"},
		{409, `{"detail":"already decommissioned"}`, ErrConflict, "conflict: already decommissioned"},
		{401, ``, ErrUnauthorized, `unauthorized; run "lfed auth login"`},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("missing bearer: %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		err := New(srv.URL+"/", "tok").Do(context.Background(), http.MethodGet, "/x", nil, nil)
		srv.Close()
		if !errors.Is(err, tc.sentinel) {
			t.Errorf("%d: errors.Is failed: %v", tc.status, err)
		}
		if err.Error() != tc.want {
			t.Errorf("%d: got %q want %q", tc.status, err.Error(), tc.want)
		}
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer srv.Close()
	err := New(srv.URL, "tok").Do(context.Background(), http.MethodGet, "/x", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 500 || apiErr.Body != "boom" {
		t.Fatalf("got %#v", err)
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("message %q", err.Error())
	}
}

func TestAPIErrorRawBodyAndValidationList(t *testing.T) {
	bodies := map[string]string{
		`plain text`: "plain text",
		`{"detail":[{"loc":["body"],"msg":"x"}]}`: `[{"loc":["body"],"msg":"x"}]`,
	}
	for body, want := range bodies {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(422)
			_, _ = w.Write([]byte(body))
		}))
		err := New(srv.URL, "tok").Do(context.Background(), http.MethodGet, "/x", nil, nil)
		srv.Close()
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Body != want {
			t.Errorf("body %q: got %#v", body, err)
		}
	}
}

func TestDoRoundTripsJSON(t *testing.T) {
	type in struct {
		Status string `json:"status"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/things/a" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("User-Agent") != "lfed-plugin/placement" {
			t.Errorf("headers %v", r.Header)
		}
		var got in
		if err := decode(r, &got); err != nil || got.Status != "Active" {
			t.Errorf("body %+v %v", got, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Cordoned"}`))
	}))
	defer srv.Close()
	var out in
	c := New(srv.URL, "tok", WithUserAgent("placement"), WithHTTPClient(srv.Client()))
	if err := c.Do(context.Background(), http.MethodPatch, "/v1/things/a", in{Status: "Active"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "Cordoned" {
		t.Errorf("got %+v", out)
	}
}

func TestDoNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	var out map[string]any
	if err := New(srv.URL, "tok").Do(context.Background(), http.MethodDelete, "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
}

func TestDoRejectsPlainHTTPRemote(t *testing.T) {
	c := New("http://api.example.test/", "tok")
	err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	want := `LFED_API_BASE_URL "http://api.example.test" must use https (plain http is allowed for loopback only)`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %s", err, want)
	}
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
