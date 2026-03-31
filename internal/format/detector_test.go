package format

import (
	"testing"

	"github.com/giobyte8/thumbnailer/internal/testutils"
)

func TestFmtDetector_DetectJpeg(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected Format
	}{
		{name: "detect jpeg in jpg  ext", filename: "1 house.jpg", expected: JPEG},
		{name: "detect jpeg in jpeg ext", filename: "2 museum.jpeg", expected: JPEG},
		{name: "detect png  in png  ext", filename: "3 screenshot.png", expected: PNG},
		{name: "detect heif in heic ext", filename: "4 thai_no_edits.heic", expected: HEIF},
		{name: "detect heif in edited f", filename: "5 thai_edited.heic", expected: HEIF},
		{name: "detect heif in jpg  ext", filename: "6 gastown_heif.jpg", expected: HEIF},
		{name: "detect webp in webp ext", filename: "7 flower.webp", expected: WEBP},

		{name: "detect mov in mov ext", filename: "10 lake_hdr.mov", expected: MOV},
		{name: "detect mp4 in mp4 ext", filename: "11 whatsapp.mp4", expected: MP4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			format := detectFmt(t, tc.filename)
			if format != tc.expected {
				t.Fatalf("expected format %v, got %v", tc.expected, format)
			}
		})
	}
}

func detectFmt(t *testing.T, filename string) Format {
	testFilePath := testutils.TestFilePath(filename)

	detector := NewFormatDetector()
	format, err := detector.Detect(testFilePath)
	if err != nil {
		t.Fatalf("failed to detect format: %v", err)
	}

	return format
}
