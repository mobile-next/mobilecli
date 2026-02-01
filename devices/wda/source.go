package wda

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
			hasIdentifier := source.Label != nil || source.Name != nil || source.RawIdentifier != nil
			alwaysInclude := source.Type == "TextField" || source.Type == "Button" || source.Type == "Switch" || source.Type == "SearchField"
			if hasIdentifier || alwaysInclude {
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

	result, err := c.getEndpointWithTimeout(endpoint, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return result, nil
}

// GetSourceRaw gets the raw page source from WDA's /source endpoint
func (c *WdaClient) GetSourceRaw() (interface{}, error) {
	startTime := time.Now()
	endpoint := "source?format=json&excluded_attributes="

	result, err := c.getEndpointWithTimeout(endpoint, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	value, ok := result["value"]
	if !ok {
		return nil, fmt.Errorf("no 'value' field found in WDA response")
	}

	elapsed := time.Since(startTime)
	utils.Verbose("GetSourceRaw took %.2f seconds", elapsed.Seconds())

	return value, nil
}

// GetSourceRawWithAttributes gets the raw page source with only the specified attributes included
func (c *WdaClient) GetSourceRawWithAttributes(attributes []string) (interface{}, error) {
	startTime := time.Now()

	// all possible attributes that can be excluded
	allAttributes := []string{
		"type", "value", "name", "label", "enabled", "visible", "accessible", "focused",
		"x", "y", "width", "height", "index", "hittable", "bundleId", "processId",
		"placeholderValue", "nativeFrame", "traits", "minValue", "maxValue", "customActions",
	}

	// build excluded list by removing requested attributes from all attributes
	excludedAttrs := []string{}
	for _, attr := range allAttributes {
		include := false
		for _, requestedAttr := range attributes {
			if attr == requestedAttr {
				include = true
				break
			}
		}
		if !include {
			excludedAttrs = append(excludedAttrs, attr)
		}
	}

	excludedStr := ""
	if len(excludedAttrs) > 0 {
		excludedStr = fmt.Sprintf("&excluded_attributes=%s", strings.Join(excludedAttrs, ","))
	}

	endpoint := fmt.Sprintf("source?format=json%s", excludedStr)

	result, err := c.getEndpointWithTimeout(endpoint, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	value, ok := result["value"]
	if !ok {
		return nil, fmt.Errorf("no 'value' field found in WDA response")
	}

	elapsed := time.Since(startTime)
	utils.Verbose("GetSourceRawWithAttributes took %.2f seconds (attributes: %v)", elapsed.Seconds(), attributes)

	return value, nil
}

func (c *WdaClient) GetSourceElements() ([]types.ScreenElement, error) {
	value, err := c.GetSourceRaw()
	// only fetch the attributes we actually use
	// value, err := c.GetSourceRawWithAttributes([]string{"type", "name", "label", "value", "visible", "x", "y", "width", "height"})
	if err != nil {
		return nil, err
	}

	utils.Verbose("WDA source response: %+v", value)

	// directly parse the value interface into our source tree struct
	var sourceTree sourceTreeElement
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := json.Unmarshal(valueBytes, &sourceTree); err != nil {
		return nil, fmt.Errorf("failed to parse source tree: %w", err)
	}

	elements := filterSourceElements(sourceTree)
	return elements, nil
}
