package devices

import "testing"

func TestScreenSizeFromLockdownValuesConvertsPixelsToPoints(t *testing.T) {
	size, err := screenSizeFromLockdownValues(uint64(1170), uint64(2532), uint64(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *size != (ScreenSize{Width: 390, Height: 844, Scale: 3}) {
		t.Fatalf("got %+v", *size)
	}
}

func TestScreenSizeFromLockdownValuesAcceptsRealScale(t *testing.T) {
	size, err := screenSizeFromLockdownValues(uint64(1640), uint64(2360), float64(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *size != (ScreenSize{Width: 820, Height: 1180, Scale: 2}) {
		t.Fatalf("got %+v", *size)
	}
}

func TestScreenSizeFromLockdownValuesRejectsMissingValues(t *testing.T) {
	if _, err := screenSizeFromLockdownValues(nil, uint64(2532), uint64(3)); err == nil {
		t.Fatal("expected error for missing width")
	}
	if _, err := screenSizeFromLockdownValues(uint64(1170), uint64(2532), uint64(0)); err == nil {
		t.Fatal("expected error for zero scale")
	}
}
