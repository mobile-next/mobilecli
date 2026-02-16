package devicekit

import (
	"fmt"

	"github.com/mobile-next/mobilecli/types"
)

func (c *Client) Tap(x, y int) error {
	params := map[string]interface{}{
		"x":        x,
		"y":        y,
		"deviceId": "",
	}
	_, err := c.call("device.io.tap", params)
	return err
}

func (c *Client) LongPress(x, y, duration int) error {
	params := map[string]interface{}{
		"x":        x,
		"y":        y,
		"duration": float64(duration) / 1000.0,
		"deviceId": "",
	}
	_, err := c.call("device.io.longpress", params)
	return err
}

func (c *Client) Swipe(x1, y1, x2, y2 int) error {
	params := map[string]interface{}{
		"fromX":    x1,
		"fromY":    y1,
		"toX":      x2,
		"toY":      y2,
		"deviceId": "",
	}
	_, err := c.call("device.io.swipe", params)
	return err
}

func (c *Client) Gesture(actions []types.TapAction) error {
	converted := convertGestureActions(actions)
	params := map[string]interface{}{
		"actions":  converted,
		"deviceId": "",
	}
	_, err := c.call("device.io.gesture", params)
	return err
}

func (c *Client) SendKeys(text string) error {
	params := map[string]interface{}{
		"text":     text,
		"deviceId": "",
	}
	_, err := c.call("device.io.text", params)
	return err
}

func (c *Client) PressButton(key string) error {
	if key == "ENTER" {
		return c.SendKeys("\n")
	}

	buttonMap := map[string]string{
		"HOME":        "home",
		"VOLUME_UP":   "volumeUp",
		"VOLUME_DOWN": "volumeDown",
	}

	translated, exists := buttonMap[key]
	if !exists {
		return fmt.Errorf("unsupported button: %s", key)
	}

	params := map[string]interface{}{
		"button":   translated,
		"deviceId": "",
	}
	_, err := c.call("device.io.button", params)
	return err
}

func (c *Client) OpenURL(url string) error {
	params := map[string]interface{}{
		"url":      url,
		"deviceId": "",
	}
	_, err := c.call("device.url", params)
	return err
}

type deviceKitAction struct {
	Type     string  `json:"type"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Duration float64 `json:"duration"`
	Button   int     `json:"button"`
}

func convertGestureActions(actions []types.TapAction) []deviceKitAction {
	var result []deviceKitAction
	var pendingDuration float64
	var lastX, lastY float64

	for i, a := range actions {
		switch a.Type {
		case "press", "move", "release":
			result = append(result, deviceKitAction{
				Type:     a.Type,
				X:        float64(a.X),
				Y:        float64(a.Y),
				Duration: float64(a.Duration) / 1000.0,
				Button:   a.Button,
			})

		case "pointerMove":
			lastX = float64(a.X)
			lastY = float64(a.Y)
			dur := float64(a.Duration)/1000.0 + pendingDuration
			pendingDuration = 0
			result = append(result, deviceKitAction{
				Type:     "move",
				X:        lastX,
				Y:        lastY,
				Duration: dur,
				Button:   a.Button,
			})

		case "pointerDown":
			x := lastX
			y := lastY
			if i+1 < len(actions) && actions[i+1].Type == "pointerMove" {
				x = float64(actions[i+1].X)
				y = float64(actions[i+1].Y)
			}
			result = append(result, deviceKitAction{
				Type:     "press",
				X:        x,
				Y:        y,
				Duration: pendingDuration,
				Button:   a.Button,
			})
			pendingDuration = 0

		case "pointerUp":
			result = append(result, deviceKitAction{
				Type:     "release",
				X:        lastX,
				Y:        lastY,
				Duration: pendingDuration,
				Button:   a.Button,
			})
			pendingDuration = 0

		case "pause":
			pendingDuration += float64(a.Duration) / 1000.0
		}
	}

	return result
}
