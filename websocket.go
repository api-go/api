package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"apigo.cc/go/cast"
	"apigo.cc/go/crypto"
	gohttp "apigo.cc/go/http"
)

const defaultWebSocketIdleTimeout = time.Minute

type managedWebSocket struct {
	key        string
	action     string
	conn       *gohttp.WebSocketConn
	lease      chan struct{}
	manager    *webSocketManager
	idleTimer  *time.Timer
	generation uint64
	mu         sync.Mutex
}

type webSocketManager struct {
	mu          sync.Mutex
	connections map[string]*managedWebSocket
}

var apiWebSockets = &webSocketManager{connections: map[string]*managedWebSocket{}}

func (manager *webSocketManager) acquire(ctx context.Context, action, connectionID, requestURL string, headers []string, timeout, idleTimeout time.Duration) (*managedWebSocket, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if idleTimeout <= 0 {
		idleTimeout = defaultWebSocketIdleTimeout
	}
	key := action + "\x00" + connectionID + "\x00" + requestURL + "\x00" + webSocketHeaderFingerprint(headers)
	manager.mu.Lock()
	managed := manager.connections[key]
	if managed == nil {
		managed = &managedWebSocket{key: key, action: action, lease: make(chan struct{}, 1), manager: manager}
		managed.lease <- struct{}{}
		manager.connections[key] = managed
	}
	manager.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-managed.lease:
	}

	managed.mu.Lock()
	if managed.idleTimer != nil {
		managed.idleTimer.Stop()
		managed.idleTimer = nil
	}
	conn := managed.conn
	managed.mu.Unlock()
	if conn != nil {
		return managed, nil
	}

	client := gohttp.NewClient(timeout)
	conn, err := client.WebSocket(requestURL, headers...)
	client.Destroy()
	if err != nil {
		managed.release(false, idleTimeout)
		return nil, err
	}
	managed.mu.Lock()
	managed.conn = conn
	managed.generation++
	managed.mu.Unlock()
	return managed, nil
}

func webSocketHeaderFingerprint(headers []string) string {
	pairs := make([]string, 0, len(headers)/2)
	for index := 1; index < len(headers); index += 2 {
		pairs = append(pairs, strings.ToLower(headers[index-1])+"\x00"+headers[index])
	}
	sort.Strings(pairs)
	return crypto.SHA256ToHex([]byte(strings.Join(pairs, "\x00")))
}

func (managed *managedWebSocket) release(healthy bool, idleTimeout time.Duration) {
	managed.mu.Lock()
	if !healthy && managed.conn != nil {
		_ = managed.conn.Close()
		managed.conn = nil
		managed.generation++
	}
	if healthy && managed.conn != nil {
		generation := managed.generation
		managed.idleTimer = time.AfterFunc(idleTimeout, func() { managed.expire(generation) })
	}
	managed.mu.Unlock()
	managed.lease <- struct{}{}
	if !healthy {
		managed.manager.removeIfEmpty(managed)
	}
}

func (managed *managedWebSocket) expire(generation uint64) {
	select {
	case <-managed.lease:
		managed.mu.Lock()
		if managed.generation == generation && managed.conn != nil {
			_ = managed.conn.Close()
			managed.conn = nil
			managed.generation++
		}
		managed.idleTimer = nil
		managed.mu.Unlock()
		managed.lease <- struct{}{}
		managed.manager.removeIfEmpty(managed)
	default:
	}
}

func (manager *webSocketManager) removeIfEmpty(managed *managedWebSocket) {
	managed.mu.Lock()
	empty := managed.conn == nil
	managed.mu.Unlock()
	if !empty {
		return
	}
	manager.mu.Lock()
	if manager.connections[managed.key] == managed {
		delete(manager.connections, managed.key)
	}
	manager.mu.Unlock()
}

func (manager *webSocketManager) invalidateAction(action string) {
	manager.mu.Lock()
	var targets []*managedWebSocket
	for key, managed := range manager.connections {
		if managed.action == action {
			delete(manager.connections, key)
			targets = append(targets, managed)
		}
	}
	manager.mu.Unlock()
	for _, managed := range targets {
		managed.mu.Lock()
		if managed.idleTimer != nil {
			managed.idleTimer.Stop()
			managed.idleTimer = nil
		}
		if managed.conn != nil {
			_ = managed.conn.Close()
			managed.conn = nil
		}
		managed.generation++
		managed.mu.Unlock()
	}
}

func isWebSocketTransport(config map[string]any) bool {
	return strings.EqualFold(cast.String(config["transport"]), "websocket")
}

func webSocketURL(value string) (string, error) {
	switch {
	case strings.HasPrefix(value, "https://"):
		return "wss://" + strings.TrimPrefix(value, "https://"), nil
	case strings.HasPrefix(value, "http://"):
		return "ws://" + strings.TrimPrefix(value, "http://"), nil
	case strings.HasPrefix(value, "wss://"), strings.HasPrefix(value, "ws://"):
		return value, nil
	default:
		return "", fmt.Errorf("WebSocket URL must use http, https, ws, or wss")
	}
}

func callWebSocket(req *HttpRequest, payload any, stream *StreamOptions, filters *filterPipeline, action string, ctx context.Context, config map[string]any, result *Result, started time.Time, timeout time.Duration) error {
	requestURL, err := webSocketURL(req.Url)
	if err != nil {
		result.Error = err.Error()
		return err
	}
	settings, _ := config["websocket"].(map[string]any)
	connectionID := cast.String(settings["connectionID"])
	idleTimeout := cast.Duration(settings["idleTimeout"])
	if idleTimeout <= 0 {
		idleTimeout = defaultWebSocketIdleTimeout
	}
	responseMode := strings.ToLower(cast.String(settings["responseMode"]))
	if responseMode == "" {
		responseMode = "first"
	}
	managed, err := apiWebSockets.acquire(ctx, action, connectionID, requestURL, headerSlice(req), timeout, idleTimeout)
	if err != nil {
		result.Error = err.Error()
		var handshakeErr *gohttp.WebSocketHandshakeError
		if errors.As(err, &handshakeErr) && handshakeErr.Response != nil {
			defer handshakeErr.Response.Body.Close()
			copyHTTPMeta(handshakeErr.Response, result)
			if body, readErr := io.ReadAll(handshakeErr.Response.Body); readErr == nil && len(body) > 0 {
				var detail any
				if cast.UnmarshalJSON(body, &detail) == nil {
					result.ErrorDetail = detail
					result.Data = detail
				}
			}
		}
		return err
	}
	healthy := false
	defer func() { managed.release(healthy, idleTimeout) }()
	conn := managed.conn
	stopCancellation := make(chan struct{})
	cancellationDone := make(chan struct{})
	var stopCancellationOnce sync.Once
	stopCancellationWatch := func() {
		stopCancellationOnce.Do(func() { close(stopCancellation) })
		<-cancellationDone
	}
	defer stopCancellationWatch()
	if ctx != nil && ctx.Done() != nil {
		go func() {
			defer close(cancellationDone)
			select {
			case <-ctx.Done():
				_ = conn.SetReadDeadline(time.Now())
			case <-stopCancellation:
			}
		}()
	} else {
		close(cancellationDone)
	}
	copyHTTPMeta(conn.Response, result)
	if result.StatusCode == 0 {
		result.StatusCode = stdhttp.StatusSwitchingProtocols
	}
	if stream != nil && stream.OnHeaders != nil {
		if err := invokeStreamCallback(func() error { return stream.OnHeaders(resultMap(result)) }); err != nil {
			result.Error = err.Error()
			return err
		}
	}
	if timeout > 0 {
		deadline := time.Now().Add(timeout)
		_ = conn.SetReadDeadline(deadline)
		_ = conn.SetWriteDeadline(deadline)
	}
	if err := conn.Send(payload); err != nil {
		result.Error = err.Error()
		return err
	}
	for {
		message, readErr := conn.ReadBytes()
		if readErr != nil {
			result.Error = readErr.Error()
			return readErr
		}
		if result.Timing != nil {
			if _, exists := result.Timing["firstToken"]; !exists {
				result.Timing["firstToken"] = time.Since(started).Milliseconds()
			}
		}
		result.receivedBytes += int64(len(message))
		filtered, filterErr := filters.apply(ctx, "chunk", map[string]any{"action": action, "chunk": message, "drop": false, "done": false})
		if filterErr != nil {
			result.Error = filterErr.Error()
			return filterErr
		}
		chunk := filteredChunk(filtered["chunk"])
		if stream != nil && stream.OnData != nil && !cast.Bool(filtered["drop"]) && len(chunk) > 0 {
			if err := invokeStreamCallback(func() error { return stream.OnData(chunk) }); err != nil {
				result.Error = err.Error()
				return err
			}
		}
		done := responseMode == "first" || cast.Bool(filtered["done"])
		if done {
			stopCancellationWatch()
			healthy = ctx == nil || ctx.Err() == nil
			_ = conn.SetReadDeadline(time.Time{})
			_ = conn.SetWriteDeadline(time.Time{})
			return nil
		}
	}
}
