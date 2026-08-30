package commands

import "testing"

func TestParseLatLon(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		lat     float64
		lon     float64
		wantErr bool
	}{
		{name: "parses a positive pair", input: "37.7749,-122.4194", lat: 37.7749, lon: -122.4194},
		{name: "parses a negative latitude", input: "-33.8688,151.2093", lat: -33.8688, lon: 151.2093},
		{name: "ignores surrounding whitespace", input: " 51.5074 , -0.1278 ", lat: 51.5074, lon: -0.1278},
		{name: "parses integers", input: "0,0", lat: 0, lon: 0},
		{name: "rejects a missing comma", input: "37.7749 -122.4194", wantErr: true},
		{name: "rejects a third component", input: "37.7749,-122.4194,10", wantErr: true},
		{name: "rejects a non-numeric latitude", input: "north,-122.4194", wantErr: true},
		{name: "rejects a non-numeric longitude", input: "37.7749,west", wantErr: true},
		{name: "rejects a latitude out of range", input: "91,0", wantErr: true},
		{name: "rejects a longitude out of range", input: "0,181", wantErr: true},
		{name: "rejects an empty string", input: "", wantErr: true},
		{name: "rejects not-a-number", input: "NaN,0", wantErr: true},
		{name: "rejects infinity", input: "0,+Inf", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon, err := ParseLatLon(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLatLon(%q) expected an error, got %v,%v", tt.input, lat, lon)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseLatLon(%q) returned unexpected error: %v", tt.input, err)
			}

			if lat != tt.lat || lon != tt.lon {
				t.Errorf("ParseLatLon(%q) = %v,%v, want %v,%v", tt.input, lat, lon, tt.lat, tt.lon)
			}
		})
	}
}
