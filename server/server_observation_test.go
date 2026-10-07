package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"heckel.io/ntfy/v2/log"
)

func observationLog(t *testing.T) func() []map[string]any {
	t.Helper()
	file, err := os.OpenFile(filepath.Join(t.TempDir(), "requests.jsonl"), os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	log.SetOutput(file)
	log.SetFormat(log.JSONFormat)
	log.SetLevel(log.TraceLevel)
	t.Cleanup(func() {
		log.SetLevel(log.ErrorLevel)
		log.SetFormat(log.TextFormat)
		log.SetOutput(os.Stderr)
		file.Close()
	})
	return func() []map[string]any {
		data, err := os.ReadFile(file.Name())
		require.NoError(t, err)
		var records []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var record map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &record))
			records = append(records, record)
		}
		return records
	}
}

func TestServer_ObservationPoll(t *testing.T) {
	records := observationLog(t)
	s := newTestServer(t, newTestConfig(t, ""))
	posted := request(t, s, "POST", "/proof", "unique test content", map[string]string{"Via": "test-proxy", "X-Request-ID": "caller-controlled"})
	require.Equal(t, 200, posted.Code)
	message := toMessage(t, posted.Body.String())
	id := posted.Header().Get("X-Request-ID")
	require.Len(t, id, 32)
	require.NotEqual(t, "caller-controlled", id)
	for _, format := range []string{"json", "sse", "raw"} {
		polled := request(t, s, "GET", "/proof/"+format+"?poll=1&since=all", "", nil,
			func(r *http.Request) { r.RemoteAddr = "8.8.8.8:4321" })
		require.Equal(t, 200, polled.Code)
		require.Contains(t, polled.Body.String(), message.Message)
		found := false
		for _, record := range records() {
			if record["observation_event"] == "subscription_message_written" && record["http_request_id"] == polled.Header().Get("X-Request-ID") {
				require.Equal(t, message.ID, record["message_id"])
				require.Equal(t, "8.8.8.8", record["visitor_ip"])
				require.Equal(t, "9.9.9.9", record["message_sender"])
				require.Len(t, record["response_chunk_sha256"], 64)
				found = true
			}
		}
		require.True(t, found, "Missing record for %s", format)
	}
	found := false
	for _, record := range records() {
		if record["http_request_id"] == id && record["http_request"] != nil {
			require.Contains(t, record["http_request"], "Via: test-proxy")
			require.Contains(t, record["http_request"], "unique test content")
			found = true
		}
	}
	require.True(t, found)
}

func TestServer_ObservationLiveReaderNotPublisher(t *testing.T) {
	records := observationLog(t)
	s := newTestServer(t, newTestConfig(t, ""))
	done := make(chan struct{})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "8.8.8.8:4321"
		defer close(done)
		s.handle(w, r)
	}))
	defer httpServer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", httpServer.URL+"/proof/json", nil)
	require.NoError(t, err)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	scan := bufio.NewScanner(resp.Body)
	require.True(t, scan.Scan()) // open event
	posted := request(t, s, "POST", "/proof", "live content", nil)
	message := toMessage(t, posted.Body.String())
	require.True(t, scan.Scan())
	require.Contains(t, scan.Text(), message.ID)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Reader failed to close")
	}
	found := false
	for _, record := range records() {
		if record["observation_event"] == "subscription_message_written" {
			require.Equal(t, "8.8.8.8", record["visitor_ip"])
			require.Equal(t, "9.9.9.9", record["message_sender"])
			require.Equal(t, resp.Header.Get("X-Request-ID"), record["http_request_id"])
			found = true
		}
	}
	require.True(t, found)
}

func TestServer_ObservationWebsocket(t *testing.T) {
	records := observationLog(t)
	s := newTestServer(t, newTestConfig(t, ""))
	posted := request(t, s, "POST", "/proof", "websocket content", nil)
	message := toMessage(t, posted.Body.String())
	done := make(chan struct{})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "8.8.8.8:4321"
		defer close(done)
		s.handle(w, r)
	}))
	defer httpServer.Close()
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/proof/ws?poll=1&since=all", nil)
	require.NoError(t, err)
	defer conn.Close()
	require.Equal(t, 101, resp.StatusCode)
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(data), message.ID)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WebSocket poll failed to close")
	}
	found := false
	for _, record := range records() {
		if record["observation_event"] == "subscription_message_written" {
			require.Equal(t, "websocket", record["subscription_transport"])
			require.Equal(t, "8.8.8.8", record["visitor_ip"])
			require.Equal(t, message.ID, record["message_id"])
			found = true
		}
	}
	require.True(t, found)
	io.Copy(io.Discard, resp.Body)
}

func TestServer_ObservationRequestBodyPreserved(t *testing.T) {
	for _, size := range []int{5000, jsonBodyBytesLimit + 100} {
		body := strings.Repeat("x", size)
		r, err := http.NewRequest("POST", "http://localhost/proof", strings.NewReader(body))
		require.NoError(t, err)
		rendered := renderHTTPRequest(r)
		reread, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, body, string(reread), "Logging must not consume or alter the handler's body")
		if size < jsonBodyBytesLimit {
			require.Contains(t, rendered, body)
		} else {
			require.Contains(t, rendered, "peeked 131072 bytes")
			require.NotContains(t, rendered, body)
		}
	}
}
