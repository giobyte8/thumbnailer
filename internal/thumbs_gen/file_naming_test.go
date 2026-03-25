package thumbsgen

import (
	"path/filepath"
	"testing"
)

func TestBaseNameNoExt(t *testing.T) {
	meta := ThumbnailMeta{
		OrigFileRelPath: filepath.Join("nested", "folder", "sample.image.png"),
	}

	got := baseNameNoExt(meta)
	if got != "sample.image" {
		t.Fatalf("unexpected base name: got %q want %q", got, "sample.image")
	}
}

func TestMkThumbFileAbsPath(t *testing.T) {
	meta := ThumbnailMeta{
		OrigFileRelPath: filepath.Join("nested", "folder", "sample.png"),
		ThumbFileAbsDir: filepath.Join("/tmp", "thumbs", "nested", "folder"),
	}

	got := mkThumbFileAbsPath(meta, 320, ".jpg")
	want := filepath.Join("/tmp", "thumbs", "nested", "folder", "sample_320px.jpg")
	if got != want {
		t.Fatalf("unexpected thumbnail path: got %q want %q", got, want)
	}
}

func TestMkDerivedFileAbsPath(t *testing.T) {
	meta := ThumbnailMeta{
		OrigFileRelPath: filepath.Join("nested", "folder", "sample.heic"),
		ThumbFileAbsDir: filepath.Join("/tmp", "thumbs", "nested", "folder"),
	}

	got := mkDerivedFileAbsPath(meta, ".jpg")
	want := filepath.Join("/tmp", "thumbs", "nested", "folder", "sample.jpg")
	if got != want {
		t.Fatalf("unexpected derived file path: got %q want %q", got, want)
	}
}
