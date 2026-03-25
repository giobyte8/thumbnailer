package thumbsgen

import "testing"

func TestThumbConstants_Smoke(t *testing.T) {
	if ThumbsQuality <= 0 {
		t.Fatalf("thumbnail quality must be > 0, got %d", ThumbsQuality)
	}
}
