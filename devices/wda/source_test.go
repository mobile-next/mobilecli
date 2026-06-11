package wda

import (
	"testing"

	"github.com/mobile-next/mobilecli/types"
)

func strPtr(s string) *string {
	return &s
}

func visibleRect(x, y, width, height float64) sourceTreeElementRect {
	return sourceTreeElementRect{X: x, Y: y, Width: width, Height: height}
}

// A realistic subtree as returned by WDA: the webview content (static texts,
// text fields, buttons) is nested inside the WebView node, separated by
// several "Other" wrapper nodes that the filter rejects.
func sampleLoginScreenTree() sourceTreeElement {
	return sourceTreeElement{
		Type: "XCUIElementTypeOther",
		Rect: visibleRect(0, 0, 402, 874),
		Children: []sourceTreeElement{
			{
				Type:  "XCUIElementTypeButton",
				Label: strPtr("Back"),
				Rect:  visibleRect(16, 62, 44, 44),
			},
			{
				Type: "XCUIElementTypeWebView",
				Rect: visibleRect(0, 116, 402, 758),
				Children: []sourceTreeElement{
					{
						Type: "XCUIElementTypeOther",
						Rect: visibleRect(0, 116, 402, 758),
						Children: []sourceTreeElement{
							{
								Type:  "XCUIElementTypeStaticText",
								Label: strPtr("Sample Login"),
								Rect:  visibleRect(24, 366, 171, 34),
							},
							{
								Type:  "XCUIElementTypeButton",
								Label: strPtr("Submit"),
								Rect:  visibleRect(24, 538, 354, 52),
							},
						},
					},
				},
			},
		},
	}
}

func elementLabel(e types.ScreenElement) string {
	if e.Label == nil {
		return ""
	}
	return *e.Label
}

func TestFilterSourceElementsNestsChildrenUnderAcceptedElements(t *testing.T) {
	output := filterSourceElements(sampleLoginScreenTree())

	if len(output) != 2 {
		t.Fatalf("expected 2 top-level elements (Back button, WebView), got %d: %+v", len(output), output)
	}

	backButton := output[0]
	if backButton.Type != "Button" || elementLabel(backButton) != "Back" {
		t.Errorf("expected first top-level element to be the Back button, got %+v", backButton)
	}

	webview := output[1]
	if webview.Type != "WebView" {
		t.Fatalf("expected second top-level element to be the WebView, got %+v", webview)
	}

	if len(webview.Children) != 2 {
		t.Fatalf("expected WebView to have 2 children (Sample Login, Submit), got %d: %+v", len(webview.Children), webview.Children)
	}

	if webview.Children[0].Type != "StaticText" || elementLabel(webview.Children[0]) != "Sample Login" {
		t.Errorf("expected first WebView child to be StaticText 'Sample Login', got %+v", webview.Children[0])
	}

	if webview.Children[1].Type != "Button" || elementLabel(webview.Children[1]) != "Submit" {
		t.Errorf("expected second WebView child to be Button 'Submit', got %+v", webview.Children[1])
	}
}

func TestFilterSourceElementsHoistsChildrenOfRejectedNodesToTopLevel(t *testing.T) {
	// Both buttons sit under rejected "Other" wrappers with no accepted
	// ancestor, so they must surface as flat top-level elements.
	tree := sourceTreeElement{
		Type: "XCUIElementTypeOther",
		Rect: visibleRect(0, 0, 402, 874),
		Children: []sourceTreeElement{
			{
				Type: "XCUIElementTypeOther",
				Rect: visibleRect(0, 0, 402, 400),
				Children: []sourceTreeElement{
					{
						Type:  "XCUIElementTypeButton",
						Label: strPtr("First"),
						Rect:  visibleRect(0, 0, 100, 50),
					},
				},
			},
			{
				Type:  "XCUIElementTypeButton",
				Label: strPtr("Second"),
				Rect:  visibleRect(0, 500, 100, 50),
			},
		},
	}

	output := filterSourceElements(tree)

	if len(output) != 2 {
		t.Fatalf("expected 2 top-level elements, got %d: %+v", len(output), output)
	}

	if elementLabel(output[0]) != "First" || elementLabel(output[1]) != "Second" {
		t.Errorf("expected buttons 'First' and 'Second' at top level, got %+v", output)
	}
}

func TestFilterSourceElementsKeepsNestedWebViewsNested(t *testing.T) {
	// WKWebView reports as three nested WebView nodes with identical rects;
	// they should nest rather than appear as three flat siblings.
	tree := sourceTreeElement{
		Type: "XCUIElementTypeWebView",
		Rect: visibleRect(0, 116, 402, 758),
		Children: []sourceTreeElement{
			{
				Type: "XCUIElementTypeWebView",
				Rect: visibleRect(0, 116, 402, 758),
				Children: []sourceTreeElement{
					{
						Type:  "XCUIElementTypeStaticText",
						Label: strPtr("Sample Login"),
						Rect:  visibleRect(24, 366, 171, 34),
					},
				},
			},
		},
	}

	output := filterSourceElements(tree)

	if len(output) != 1 {
		t.Fatalf("expected 1 top-level WebView, got %d: %+v", len(output), output)
	}

	outer := output[0]
	if len(outer.Children) != 1 || outer.Children[0].Type != "WebView" {
		t.Fatalf("expected outer WebView to contain the inner WebView, got %+v", outer.Children)
	}

	inner := outer.Children[0]
	if len(inner.Children) != 1 || elementLabel(inner.Children[0]) != "Sample Login" {
		t.Errorf("expected inner WebView to contain StaticText 'Sample Login', got %+v", inner.Children)
	}
}

func TestFilterSourceElementsOmitsChildrenFromJsonWhenEmpty(t *testing.T) {
	// Leaf elements must not serialize an empty "children" array, so the
	// JSON output stays unchanged for consumers that expect leaves.
	output := filterSourceElements(sourceTreeElement{
		Type:  "XCUIElementTypeButton",
		Label: strPtr("Back"),
		Rect:  visibleRect(16, 62, 44, 44),
	})

	if len(output) != 1 {
		t.Fatalf("expected 1 element, got %d", len(output))
	}

	if output[0].Children != nil {
		t.Errorf("expected leaf element to have nil Children, got %+v", output[0].Children)
	}
}
