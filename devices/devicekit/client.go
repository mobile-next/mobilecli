package devicekit

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/utils"
)

type Client struct {
	httpURL    string
	wsURL      string
	httpClient *http.Client
	requestID  atomic.Int64

	mu       sync.Mutex
	conn     *websocket.Conn
	pending  map[int64]chan jsonRPCResponse
	closeErr error
}

func NewClient(hostname string, port int) *Client {
	httpURL := fmt.Sprintf("http://%s:%d", hostname, port)
	wsURL := fmt.Sprintf("ws://%s:%d", hostname, port)

	return &Client{
		httpURL: httpURL,
		wsURL:   wsURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		pending: make(map[int64]chan jsonRPCResponse),
	}
}

func (c *Client) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	url := fmt.Sprintf("%s/rpc", c.wsURL)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to DeviceKit WebSocket: %w", err)
	}

	c.conn = conn
	c.closeErr = nil
	go c.readLoop(conn)

	return nil
}

func (c *Client) readLoop(conn *websocket.Conn) {
	for {
		var resp jsonRPCResponse
		err := conn.ReadJSON(&resp)
		if err != nil {
			c.mu.Lock()
			c.closeErr = err
			c.conn = nil
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = make(map[int64]chan jsonRPCResponse)
			c.mu.Unlock()
			return
		}

		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()

		if ok {
			ch <- resp
		}
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[int64]chan jsonRPCResponse)
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	for _, ch := range pending {
		close(ch)
	}
}

func (c *Client) HealthCheck() error {
	url := fmt.Sprintf("%s/health", c.httpURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) WaitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for DeviceKit to be ready")
		case <-ticker.C:
			err := c.HealthCheck()
			if err != nil {
				utils.Verbose("DeviceKit not ready yet: %v", err)
				continue
			}
			utils.Verbose("DeviceKit is ready!")
			return nil
		}
	}
}
