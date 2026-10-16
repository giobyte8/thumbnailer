package main

import "testing"

func TestParseMaxAllowedImageDimension(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "positive integer", value: "4096", want: 4096},
		{name: "missing", wantErr: true},
		{name: "invalid integer", value: "large", wantErr: true},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseMaxAllowedImageDimension(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseMaxAllowedImageDimension(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("parseMaxAllowedImageDimension(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestValidateThumbnailWidth(t *testing.T) {
	if err := validateThumbnailWidth(4096, 4096); err != nil {
		t.Fatalf("width at maximum should be accepted: %v", err)
	}
	if err := validateThumbnailWidth(4097, 4096); err == nil {
		t.Fatal("width above maximum should be rejected")
	}
}
