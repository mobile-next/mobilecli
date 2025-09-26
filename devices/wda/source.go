package wda

import (
	"encoding/json"
	"fmt"

	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

// sourceTreeElementRect represents the rect structure from WDA source tree
type sourceTreeElementRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// sourceTreeElement represents an element from WDA source tree
type sourceTreeElement struct {
	Type          string                  `json:"type"`
	Label         *string                 `json:"label"`
	Name          *string                 `json:"name"`
	Value         *string                 `json:"value"`
	RawIdentifier *string                 `json:"rawIdentifier"`
	IsVisible     string                  `json:"isVisible"`
	Rect          sourceTreeElementRect   `json:"rect"`
	Children      []sourceTreeElement     `json:"children"`
}


func isVisible(rect sourceTreeElementRect) bool {
	return rect.X >= 0 && rect.Y >= 0
}


func filterSourceElements(source sourceTreeElement) []types.ScreenElement {
	var output []types.ScreenElement

	acceptedTypes := []string{"TextField", "Button", "Switch", "Icon", "SearchField", "StaticText", "Image"}

	typeAccepted := false
	for _, acceptedType := range acceptedTypes {
		if source.Type == acceptedType {
			typeAccepted = true
			break
		}
	}

	if typeAccepted {
		if source.IsVisible == "1" && isVisible(source.Rect) {
			if source.Label != nil || source.Name != nil || source.RawIdentifier != nil {
				output = append(output, types.ScreenElement{
					Type:       source.Type,
					Label:      source.Label,
					Name:       source.Name,
					Value:      source.Value,
					Identifier: source.RawIdentifier,
					Rect: types.ScreenElementRect{
						X:      source.Rect.X,
						Y:      source.Rect.Y,
						Width:  source.Rect.Width,
						Height: source.Rect.Height,
					},
				})
			}
		}
	}

	for _, child := range source.Children {
		childElements := filterSourceElements(child)
		output = append(output, childElements...)
	}

	return output
}

// GetSource gets the page source from WDA's /source endpoint with format=json
func (c *WdaClient) GetSource() (map[string]interface{}, error) {
	endpoint := "source?format=json"

	result, err := c.GetEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %v", err)
	}

	return result, nil
}

func (c *WdaClient) GetSourceElements() ([]types.ScreenElement, error) {
	endpoint := "source?format=json"

	result, err := c.GetEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %v", err)
	}

	utils.Verbose("WDA source response: %+v", result)

	valueInterface, ok := result["value"]
	if !ok {
		return nil, fmt.Errorf("no 'value' field found in WDA response")
	}

	// directly parse the value interface into our source tree struct
	var sourceTree sourceTreeElement
	valueBytes, err := json.Marshal(valueInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value: %v", err)
	}

	if err := json.Unmarshal(valueBytes, &sourceTree); err != nil {
		return nil, fmt.Errorf("failed to parse source tree: %v", err)
	}

	elements := filterSourceElements(sourceTree)
	return elements, nil
}

