package devauth

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type cdpClient struct {
	ws      *wsConn
	mu      sync.Mutex
	nextID  int
	pending map[int]chan cdpResponse
	events  chan cdpEvent
	done    chan struct{}
}

type cdpEvent struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type cdpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newCDPClient(ctx context.Context, endpoint string) (*cdpClient, error) {
	ws, err := dialWebSocket(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("dev auth: connect Chrome DevTools: %w", err)
	}
	client := &cdpClient{
		ws:      ws,
		pending: map[int]chan cdpResponse{},
		events:  make(chan cdpEvent, 128),
		done:    make(chan struct{}),
	}
	go client.readLoop()
	return client, nil
}

func (c *cdpClient) Close() {
	_ = c.ws.Close()
	<-c.done
}

func (c *cdpClient) Events() <-chan cdpEvent {
	return c.events
}

func (c *cdpClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan cdpResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	message := map[string]any{
		"id":     id,
		"method": method,
	}
	if params != nil {
		message["params"] = params
	}
	data, err := json.Marshal(message)
	if err != nil {
		c.removePending(id)
		return nil, err
	}
	if err := c.ws.WriteText(data); err != nil {
		c.removePending(id)
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.removePending(id)
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("cdp %s: connection closed", method)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("cdp %s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *cdpClient) removePending(id int) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *cdpClient) readLoop() {
	defer close(c.done)
	defer close(c.events)
	for {
		payload, err := c.ws.ReadText()
		if err != nil {
			c.failPending()
			return
		}
		var probe struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(payload, &probe); err != nil {
			continue
		}
		if probe.ID != 0 {
			var resp cdpResponse
			if err := json.Unmarshal(payload, &resp); err != nil {
				continue
			}
			c.mu.Lock()
			ch := c.pending[resp.ID]
			delete(c.pending, resp.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- resp
			}
			continue
		}
		if probe.Method != "" {
			select {
			case c.events <- cdpEvent{Method: probe.Method, Params: probe.Params}:
			default:
			}
		}
	}
}

func (c *cdpClient) failPending() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		delete(c.pending, id)
		close(ch)
	}
}
