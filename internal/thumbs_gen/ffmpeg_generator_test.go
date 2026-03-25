package thumbsgen

import (
	"reflect"
	"testing"
)

func TestIsFFmpegSupported(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{name: "mp4", filePath: "video.mp4", want: true},
		{name: "mov upper", filePath: "video.MOV", want: true},
		{name: "heic unsupported", filePath: "folder/image.heic", want: false},
		{name: "png unsupported", filePath: "image.png", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFFmpegSupported(tt.filePath)
			if got != tt.want {
				t.Fatalf("unexpected support result for %q: got %v want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestMkFFmpegArgs(t *testing.T) {
	generator := &FFmpegThumbsGenerator{}

	got := generator.mkFFmpegArgs("/tmp/in.mov", "/tmp/out.webp", 320)
	want := []string{
		"-y",
		"-ss", "00:00:01",
		"-i", "/tmp/in.mov",
		"-frames:v", "1",
		"-vf", "scale='min(320,iw)':-2",
		"-q:v", "60",
		"/tmp/out.webp",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ffmpeg args: got %v want %v", got, want)
	}
}
