package devices

import "testing"

func TestFlattenElementsIncludesNestedChildren(t *testing.T) {
	name := "BroadcastUploadExtension"
	tree := []ScreenElement{
		{
			Type: "Window",
			Children: []ScreenElement{
				{Type: "Button", Name: &name},
			},
		},
	}

	flat := flattenElements(tree)

	if len(flat) != 2 {
		t.Fatalf("expected 2 elements after flatten, got %d", len(flat))
	}
	if flat[1].Name == nil || *flat[1].Name != name {
		t.Errorf("expected nested button to be flattened, got %+v", flat[1])
	}
}

func button(name string, x, y, width, height int) ScreenElement {
	return ScreenElement{
		Type:  "Button",
		Name:  &name,
		Label: &name,
		Rect:  ScreenElementRect{X: x, Y: y, Width: width, Height: height},
	}
}

// the devicekit app shows a single record button, which the broadcast picker
// view names "ModuleIcon"
func recordButton() ScreenElement { return button("ModuleIcon", 172, 406, 80, 80) }

// when the app was opened from another app, iOS adds a "Return to <app>" button
// to the status bar
func breadcrumbButton() ScreenElement { return button("Return to Settings", 12, 31, 57, 13) }

func startBroadcastingLabel() ScreenElement {
	text := "Press to Start Broadcasting"
	return ScreenElement{Type: "StaticText", Label: &text, Name: &text}
}

func TestFindRecordButtonIgnoresTheReturnToAppBreadcrumb(t *testing.T) {
	found, err := findRecordButton([]ScreenElement{breadcrumbButton(), recordButton(), startBroadcastingLabel()})

	if err != nil {
		t.Fatalf("expected the record button to be found, got error: %v", err)
	}
	if found.Name == nil || *found.Name != "ModuleIcon" {
		t.Errorf("expected the record button, got %+v", found)
	}
}

func TestFindRecordButtonAcceptsTheOnlyButtonWhateverItIsCalled(t *testing.T) {
	found, err := findRecordButton([]ScreenElement{button("Record", 172, 406, 80, 80), startBroadcastingLabel()})

	if err != nil {
		t.Fatalf("expected the only button to be used, got error: %v", err)
	}
	if found.Name == nil || *found.Name != "Record" {
		t.Errorf("expected the only button, got %+v", found)
	}
}

func TestFindRecordButtonFailsWhenItCannotTellWhichButtonRecords(t *testing.T) {
	_, err := findRecordButton([]ScreenElement{breadcrumbButton(), button("Record", 172, 406, 80, 80)})
	if err == nil {
		t.Error("expected an error for two buttons, neither of them the record button")
	}

	_, err = findRecordButton([]ScreenElement{startBroadcastingLabel()})
	if err == nil {
		t.Error("expected an error when there is no button at all")
	}
}
