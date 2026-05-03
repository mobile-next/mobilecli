package wda

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

type sourceTreeElementRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type sourceTreeElement struct {
	Type             string                `json:"type"`
	Label            *string               `json:"label"`
	Name             *string               `json:"name"`
	Value            *string               `json:"value"`
	PlaceholderValue *string               `json:"placeholderValue"`
	RawIdentifier    *string               `json:"rawIdentifier"`
	Rect             sourceTreeElementRect `json:"rect"`
	Children         []sourceTreeElement   `json:"children"`
}

func isVisible(rect sourceTreeElementRect) bool {
	return rect.X >= 0 && rect.Y >= 0 && rect.Width > 0 && rect.Height > 0
}

func filterSourceElements(source sourceTreeElement) []types.ScreenElement {
	var output []types.ScreenElement

	acceptedTypes := []string{"TextField", "Button", "Switch", "Icon", "SearchField", "StaticText", "Image", "SecureTextField"}

	// strip XCUIElementType prefix if present
	elementType := strings.TrimPrefix(source.Type, "XCUIElementType")

	typeAccepted := false
	for _, acceptedType := range acceptedTypes {
		if elementType == acceptedType {
			typeAccepted = true
			break
		}
	}

	if typeAccepted {
		if isVisible(source.Rect) {
			hasIdentifier := source.Label != nil || source.Name != nil || source.RawIdentifier != nil || source.PlaceholderValue != nil
			alwaysInclude := elementType == "TextField" || elementType == "SecureTextField" || elementType == "Button" || elementType == "Switch" || elementType == "SearchField"
			if hasIdentifier || alwaysInclude {
				output = append(output, types.ScreenElement{
					Type:        elementType,
					Label:       source.Label,
					Name:        source.Name,
					Value:       source.Value,
					Placeholder: source.PlaceholderValue,
					Identifier:  source.RawIdentifier,
					Rect: types.ScreenElementRect{
						X:      int(source.Rect.X),
						Y:      int(source.Rect.Y),
						Width:  int(source.Rect.Width),
						Height: int(source.Rect.Height),
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

func (c *WdaClient) GetSourceRaw() (any, error) {
	startTime := time.Now()

	result, err := c.CallRPC("device.dump.ui", map[string]string{"format": "raw"})
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	var value any
	if err := json.Unmarshal(result, &value); err != nil {
		return nil, fmt.Errorf("failed to parse source: %w", err)
	}

	elapsed := time.Since(startTime)
	utils.Verbose("GetSourceRaw took %.2f seconds", elapsed.Seconds())

	return value, nil
}

func (c *WdaClient) GetSourceElements() ([]types.ScreenElement, error) {
	startTime := time.Now()

	result, err := c.CallRPC("device.dump.ui", map[string]string{"format": "json"})
	if err != nil {
		return nil, err
	}

	var sourceTree sourceTreeElement
	if err := json.Unmarshal(result, &sourceTree); err != nil {
		return nil, fmt.Errorf("failed to parse source tree: %w", err)
	}

	elapsed := time.Since(startTime)
	utils.Verbose("GetSourceElements took %.2f seconds", elapsed.Seconds())

	elements := filterSourceElements(sourceTree)
	return elements, nil
}
