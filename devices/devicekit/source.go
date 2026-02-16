package devicekit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

type axFrame struct {
	X      float64 `json:"X"`
	Y      float64 `json:"Y"`
	Width  float64 `json:"Width"`
	Height float64 `json:"Height"`
}

type axElement struct {
	Identifier       string      `json:"identifier"`
	Frame            axFrame     `json:"frame"`
	Value            *string     `json:"value"`
	Title            *string     `json:"title"`
	Label            string      `json:"label"`
	ElementType      int         `json:"elementType"`
	Enabled          bool        `json:"enabled"`
	PlaceholderValue *string     `json:"placeholderValue"`
	Children         []axElement `json:"children"`
}

type viewHierarchy struct {
	AXElement axElement `json:"axElement"`
	Depth     int       `json:"depth"`
}

var elementTypeMap = map[int]string{
	9:  "Button",
	40: "Switch",
	48: "TextField",
	49: "SearchField",
	52: "StaticText",
	57: "Icon",
	69: "Image",
}

var acceptedTypes = map[string]bool{
	"Button":      true,
	"TextField":   true,
	"Switch":      true,
	"Icon":        true,
	"SearchField": true,
	"StaticText":  true,
	"Image":       true,
}

var alwaysIncludeTypes = map[string]bool{
	"Button":      true,
	"TextField":   true,
	"Switch":      true,
	"SearchField": true,
}

func (c *Client) GetSourceElements() ([]types.ScreenElement, error) {
	startTime := time.Now()

	params := map[string]interface{}{
		"deviceId": "",
		"format":   "json",
	}

	result, err := c.callWithTimeout("device.dump.ui", params, 60*time.Second)
	if err != nil {
		return nil, err
	}

	var hierarchy viewHierarchy
	if err := json.Unmarshal(result, &hierarchy); err != nil {
		return nil, fmt.Errorf("failed to parse dump response: %w", err)
	}

	elements := filterAXElements(hierarchy.AXElement)

	elapsed := time.Since(startTime)
	utils.Verbose("GetSourceElements took %.2f seconds", elapsed.Seconds())

	return elements, nil
}

func (c *Client) GetSourceRaw() (interface{}, error) {
	startTime := time.Now()

	params := map[string]interface{}{
		"deviceId": "",
		"format":   "json",
	}

	result, err := c.callWithTimeout("device.dump.ui", params, 60*time.Second)
	if err != nil {
		return nil, err
	}

	var raw interface{}
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse dump response: %w", err)
	}

	elapsed := time.Since(startTime)
	utils.Verbose("GetSourceRaw took %.2f seconds", elapsed.Seconds())

	return raw, nil
}

func filterAXElements(element axElement) []types.ScreenElement {
	var output []types.ScreenElement

	typeName := elementTypeMap[element.ElementType]

	if acceptedTypes[typeName] {
		visible := element.Enabled && element.Frame.X >= 0 && element.Frame.Y >= 0

		if visible {
			hasIdentifier := element.Label != "" || element.Identifier != ""
			if hasIdentifier || alwaysIncludeTypes[typeName] {
				var label *string
				if element.Label != "" {
					label = &element.Label
				}

				var identifier *string
				if element.Identifier != "" {
					identifier = &element.Identifier
				}

				var name *string
				if element.Label != "" {
					name = &element.Label
				}

				output = append(output, types.ScreenElement{
					Type:       typeName,
					Label:      label,
					Name:       name,
					Value:      element.Value,
					Identifier: identifier,
					Rect: types.ScreenElementRect{
						X:      int(element.Frame.X),
						Y:      int(element.Frame.Y),
						Width:  int(element.Frame.Width),
						Height: int(element.Frame.Height),
					},
				})
			}
		}
	}

	for _, child := range element.Children {
		childElements := filterAXElements(child)
		output = append(output, childElements...)
	}

	return output
}
