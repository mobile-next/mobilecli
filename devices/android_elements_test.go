package devices

import (
	"testing"

	"github.com/mobile-next/mobilecli/types"
)

func screenElementText(e types.ScreenElement) string {
	if e.Text == nil {
		return ""
	}
	return *e.Text
}

func screenElementPlaceholder(e types.ScreenElement) string {
	if e.Placeholder == nil {
		return ""
	}
	return *e.Placeholder
}

// A realistic uiautomator subtree: the webview content (text views, buttons)
// is nested inside the android.webkit.WebView node, separated by layout
// wrapper nodes that carry no text, content-desc, hint or resource-id and are
// therefore filtered out.
func sampleLoginScreenXmlTree() uiAutomatorXmlNode {
	return uiAutomatorXmlNode{
		Class:  "android.widget.FrameLayout",
		Bounds: "[0,0][1080,2400]",
		Nodes: []uiAutomatorXmlNode{
			{
				Class:  "android.widget.Button",
				Text:   "Back",
				Bounds: "[40,150][150,260]",
			},
			{
				Class:      "android.webkit.WebView",
				ResourceID: "com.mobilenext.playground:id/webview",
				Bounds:     "[0,290][1080,2300]",
				Nodes: []uiAutomatorXmlNode{
					{
						Class:  "android.view.View",
						Bounds: "[0,290][1080,2300]",
						Nodes: []uiAutomatorXmlNode{
							{
								Class:  "android.widget.TextView",
								Text:   "Sample Login",
								Bounds: "[60,900][490,990]",
							},
							{
								Class:  "android.widget.Button",
								Text:   "Submit",
								Bounds: "[60,1340][940,1470]",
							},
						},
					},
				},
			},
		},
	}
}

func TestCollectElementsNestsChildrenUnderAcceptedElements(t *testing.T) {
	d := &AndroidDevice{}
	output := d.collectElements(sampleLoginScreenXmlTree())

	if len(output) != 2 {
		t.Fatalf("expected 2 top-level elements (Back button, WebView), got %d: %+v", len(output), output)
	}

	backButton := output[0]
	if backButton.Type != "android.widget.Button" || screenElementText(backButton) != "Back" {
		t.Errorf("expected first top-level element to be the Back button, got %+v", backButton)
	}

	webview := output[1]
	if webview.Type != "android.webkit.WebView" {
		t.Fatalf("expected second top-level element to be the WebView, got %+v", webview)
	}

	if len(webview.Children) != 2 {
		t.Fatalf("expected WebView to have 2 children (Sample Login, Submit), got %d: %+v", len(webview.Children), webview.Children)
	}

	if screenElementText(webview.Children[0]) != "Sample Login" {
		t.Errorf("expected first WebView child to be 'Sample Login', got %+v", webview.Children[0])
	}

	if screenElementText(webview.Children[1]) != "Submit" {
		t.Errorf("expected second WebView child to be 'Submit', got %+v", webview.Children[1])
	}
}

func TestCollectElementsHoistsChildrenOfRejectedNodesToTopLevel(t *testing.T) {
	d := &AndroidDevice{}

	tree := uiAutomatorXmlNode{
		Class:  "android.widget.FrameLayout",
		Bounds: "[0,0][1080,2400]",
		Nodes: []uiAutomatorXmlNode{
			{
				Class:  "android.widget.LinearLayout",
				Bounds: "[0,0][1080,1200]",
				Nodes: []uiAutomatorXmlNode{
					{
						Class:  "android.widget.Button",
						Text:   "First",
						Bounds: "[0,0][200,100]",
					},
				},
			},
			{
				Class:  "android.widget.Button",
				Text:   "Second",
				Bounds: "[0,1300][200,1400]",
			},
		},
	}

	output := d.collectElements(tree)

	if len(output) != 2 {
		t.Fatalf("expected 2 top-level elements, got %d: %+v", len(output), output)
	}

	if screenElementText(output[0]) != "First" || screenElementText(output[1]) != "Second" {
		t.Errorf("expected buttons 'First' and 'Second' at top level, got %+v", output)
	}
}

func TestCollectDeviceKitElementsNestsChildrenUnderAcceptedElements(t *testing.T) {
	nodes := []deviceKitNode{
		{
			Class: "android.widget.FrameLayout",
			Rect:  deviceKitRect{X: 0, Y: 0, Width: 1080, Height: 2400},
			Children: []deviceKitNode{
				{
					Class:      "android.webkit.WebView",
					ResourceID: "com.mobilenext.playground:id/webview",
					Rect:       deviceKitRect{X: 0, Y: 290, Width: 1080, Height: 2010},
					Children: []deviceKitNode{
						{
							Class: "android.view.View",
							Rect:  deviceKitRect{X: 0, Y: 290, Width: 1080, Height: 2010},
							Children: []deviceKitNode{
								{
									Class: "android.widget.TextView",
									Text:  "Sample Login",
									Rect:  deviceKitRect{X: 60, Y: 900, Width: 430, Height: 90},
								},
							},
						},
					},
				},
			},
		},
	}

	output := collectDeviceKitElements(nodes)

	if len(output) != 1 {
		t.Fatalf("expected 1 top-level element (WebView), got %d: %+v", len(output), output)
	}

	webview := output[0]
	if webview.Type != "android.webkit.WebView" {
		t.Fatalf("expected top-level element to be the WebView, got %+v", webview)
	}

	if len(webview.Children) != 1 || screenElementText(webview.Children[0]) != "Sample Login" {
		t.Errorf("expected WebView child 'Sample Login', got %+v", webview.Children)
	}
}

func TestCollectDeviceKitElementsHoistsChildrenOfRejectedNodesToTopLevel(t *testing.T) {
	nodes := []deviceKitNode{
		{
			Class: "android.widget.LinearLayout",
			Rect:  deviceKitRect{X: 0, Y: 0, Width: 1080, Height: 1200},
			Children: []deviceKitNode{
				{
					Class: "android.widget.Button",
					Text:  "First",
					Rect:  deviceKitRect{X: 0, Y: 0, Width: 200, Height: 100},
				},
			},
		},
		{
			Class: "android.widget.Button",
			Text:  "Second",
			Rect:  deviceKitRect{X: 0, Y: 1300, Width: 200, Height: 100},
		},
	}

	output := collectDeviceKitElements(nodes)

	if len(output) != 2 {
		t.Fatalf("expected 2 top-level elements, got %d: %+v", len(output), output)
	}

	if screenElementText(output[0]) != "First" || screenElementText(output[1]) != "Second" {
		t.Errorf("expected buttons 'First' and 'Second' at top level, got %+v", output)
	}
}

// The hint becomes the placeholder; the text is left exactly as the source
// reported it, even when it happens to equal the hint.
func TestCollectElementsHintBecomesPlaceholderAndKeepsText(t *testing.T) {
	d := &AndroidDevice{}
	tree := uiAutomatorXmlNode{
		Class:       "android.widget.EditText",
		Text:        "Password",
		Hint:        "Password",
		ContentDesc: "password_field",
		Bounds:      "[48,607][1232,756]",
	}

	output := d.collectElements(tree)

	if len(output) != 1 {
		t.Fatalf("expected 1 element, got %d: %+v", len(output), output)
	}
	if got := screenElementPlaceholder(output[0]); got != "Password" {
		t.Errorf("expected placeholder 'Password', got %q", got)
	}
	if got := screenElementText(output[0]); got != "Password" {
		t.Errorf("expected text left as source reported ('Password'), got %q", got)
	}
}

// A filled field keeps its real text alongside the hint placeholder.
func TestCollectElementsFilledFieldKeepsTextAndPlaceholder(t *testing.T) {
	d := &AndroidDevice{}
	tree := uiAutomatorXmlNode{
		Class:       "android.widget.EditText",
		Text:        "hello",
		Hint:        "Text Field",
		ContentDesc: "text_field",
		Bounds:      "[48,455][1232,604]",
	}

	output := d.collectElements(tree)

	if len(output) != 1 {
		t.Fatalf("expected 1 element, got %d: %+v", len(output), output)
	}
	if got := screenElementText(output[0]); got != "hello" {
		t.Errorf("expected text 'hello', got %q", got)
	}
	if got := screenElementPlaceholder(output[0]); got != "Text Field" {
		t.Errorf("expected placeholder 'Text Field', got %q", got)
	}
}

// devicekit sends a separate hint field; it becomes the placeholder, and the
// masked password text is preserved.
func TestCollectDeviceKitElementsHintBecomesPlaceholder(t *testing.T) {
	nodes := []deviceKitNode{
		{
			Class:       "android.widget.EditText",
			Text:        "•",
			Hint:        "Password",
			ContentDesc: "password_field",
			ResourceID:  "com.mobilenext.playground:id/password_field",
			Rect:        deviceKitRect{X: 48, Y: 607, Width: 1184, Height: 149},
		},
	}

	output := collectDeviceKitElements(nodes)

	if len(output) != 1 {
		t.Fatalf("expected 1 element, got %d: %+v", len(output), output)
	}
	if got := screenElementPlaceholder(output[0]); got != "Password" {
		t.Errorf("expected placeholder 'Password', got %q", got)
	}
	if got := screenElementText(output[0]); got != "•" {
		t.Errorf("expected text '•', got %q", got)
	}
}
