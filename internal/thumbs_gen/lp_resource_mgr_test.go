package thumbsgen

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"
	"time"

	"github.com/discord/lilliput"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
)

func TestLPResourceManagerUsesDefaultRetentionWhenUnset(t *testing.T) {
	value, wasSet := os.LookupEnv(lpResourcesRetentionEnv)
	if err := os.Unsetenv(lpResourcesRetentionEnv); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(lpResourcesRetentionEnv, value)
		} else {
			_ = os.Unsetenv(lpResourcesRetentionEnv)
		}
	})

	manager, err := NewLPResourceManager(testTelemetry(t))
	if err != nil {
		t.Fatalf("NewLPResourceManager() error = %v", err)
	}
	defer manager.Close()

	if manager.retentionPeriod != 5*time.Minute {
		t.Fatalf("retention period = %s, want 5m", manager.retentionPeriod)
	}
	if manager.imageOps != nil || manager.resizeBuffer != nil {
		t.Fatal("resources should not be allocated before the first transform")
	}
}

func TestLPResourceManagerRejectsRetentionOverflow(t *testing.T) {
	t.Setenv(lpResourcesRetentionEnv, "9223372036854775807")
	if _, err := NewLPResourceManager(testTelemetry(t)); err == nil {
		t.Fatal("NewLPResourceManager() should reject a duration that overflows time.Duration")
	}
}

func TestLPResourceManagerUsesConfiguredRetention(t *testing.T) {
	t.Setenv("LP_RESOURCES_MEMORY_RETENTION_PERIOD", "12")

	manager, err := NewLPResourceManager(testTelemetry(t))
	if err != nil {
		t.Fatalf("NewLPResourceManager() error = %v", err)
	}
	defer manager.Close()

	if manager.retentionPeriod != 12*time.Second {
		t.Fatalf("retention period = %s, want 12s", manager.retentionPeriod)
	}
}

func TestLPResourceManagerRejectsInvalidRetention(t *testing.T) {
	for _, value := range []string{"invalid", "0", "-1"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("LP_RESOURCES_MEMORY_RETENTION_PERIOD", value)
			if _, err := NewLPResourceManager(testTelemetry(t)); err == nil {
				t.Fatalf("NewLPResourceManager() with %q should fail", value)
			}
		})
	}
}

func TestLPResourceManagerAllocatesAndExpandsOnDemand(t *testing.T) {
	manager := newTestLPResourceManager(t, time.Hour)
	defer manager.Close()

	manager.mu.Lock()
	manager.ensureCapacityLocked(8)
	manager.mu.Unlock()
	if manager.imageOpsMaxSize != 8 || manager.imageOps == nil {
		t.Fatalf("initial ImageOps capacity = %d, want 8", manager.imageOpsMaxSize)
	}
	if len(manager.resizeBuffer) != lpResizeBufferSize {
		t.Fatalf("resize buffer size = %d, want %d", len(manager.resizeBuffer), lpResizeBufferSize)
	}

	manager.mu.Lock()
	manager.ensureCapacityLocked(16)
	manager.mu.Unlock()
	if manager.imageOpsMaxSize != 16 {
		t.Fatalf("expanded ImageOps capacity = %d, want 16", manager.imageOpsMaxSize)
	}
	if len(manager.resizeBuffer) != lpResizeBufferSize {
		t.Fatalf("resize buffer size after expansion = %d, want %d", len(manager.resizeBuffer), lpResizeBufferSize)
	}
}

func TestLPResourceManagerTransformReturnsOwnedBytesAndAllowsReuse(t *testing.T) {
	manager := newTestLPResourceManager(t, time.Hour)
	defer manager.Close()

	imageBytes := makeTestJPEG(t)
	newDecoder := func() lilliput.Decoder {
		decoder, err := lilliput.NewDecoder(imageBytes)
		if err != nil {
			t.Fatalf("NewDecoder() error = %v", err)
		}
		return decoder
	}
	options := &lilliput.ImageOptions{
		FileType:              ".webp",
		Width:                 8,
		Height:                4,
		ResizeMethod:          lilliput.ImageOpsFit,
		DisableAnimatedOutput: true,
		EncodeTimeout:         time.Second,
	}

	firstDecoder := newDecoder()
	firstResult, err := manager.Transform(firstDecoder, options, 16)
	firstDecoder.Close()
	if err != nil {
		t.Fatalf("first Transform() error = %v", err)
	}
	firstResultCopy := append([]byte(nil), firstResult...)

	manager.mu.Lock()
	for index := range manager.resizeBuffer[:len(firstResult)] {
		manager.resizeBuffer[index] = 0
	}
	manager.mu.Unlock()
	if !bytes.Equal(firstResult, firstResultCopy) {
		t.Fatal("returned bytes changed when the reusable output buffer was overwritten")
	}

	secondDecoder := newDecoder()
	_, err = manager.Transform(secondDecoder, options, 8)
	secondDecoder.Close()
	if err != nil {
		t.Fatalf("second Transform() after buffer clear error = %v", err)
	}
}

func TestLPResourceManagerShrinksAfterLowUtilizationWindow(t *testing.T) {
	manager := newTestLPResourceManager(t, time.Hour)
	defer manager.Close()

	manager.mu.Lock()
	manager.ensureCapacityLocked(100)
	manager.periodMaxRequiredSize = 80
	manager.lastTransformTime = time.Now()
	manager.maintainResourcesLocked(time.Now())
	manager.mu.Unlock()

	if manager.imageOpsMaxSize != 80 {
		t.Fatalf("ImageOps capacity after low-use window = %d, want 80", manager.imageOpsMaxSize)
	}
	if manager.periodMaxRequiredSize != 0 {
		t.Fatalf("period maximum after evaluation = %d, want 0", manager.periodMaxRequiredSize)
	}
}

func TestLPResourceManagerRetainsCapacityAtNinetyPercent(t *testing.T) {
	manager := newTestLPResourceManager(t, time.Hour)
	defer manager.Close()

	manager.mu.Lock()
	manager.ensureCapacityLocked(100)
	manager.periodMaxRequiredSize = 90
	manager.lastTransformTime = time.Now()
	manager.maintainResourcesLocked(time.Now())
	manager.mu.Unlock()

	if manager.imageOpsMaxSize != 100 {
		t.Fatalf("ImageOps capacity at 90%% use = %d, want 100", manager.imageOpsMaxSize)
	}
}

func TestLPResourceManagerReleasesResourcesWhenIdle(t *testing.T) {
	manager := newTestLPResourceManager(t, time.Minute)
	manager.mu.Lock()
	manager.ensureCapacityLocked(8)
	manager.lastTransformTime = time.Now().Add(-time.Minute)
	manager.mu.Unlock()
	defer manager.Close()

	manager.mu.Lock()
	manager.maintainResourcesLocked(time.Now())
	manager.mu.Unlock()

	if manager.imageOps != nil || manager.resizeBuffer != nil {
		t.Fatal("idle cleanup should release ImageOps and resize buffer")
	}
	if manager.imageOpsMaxSize != 0 || manager.periodMaxRequiredSize != 0 {
		t.Fatalf("idle cleanup left stale size state: capacity=%d periodMax=%d", manager.imageOpsMaxSize, manager.periodMaxRequiredSize)
	}
}

func TestLPResourceManagerTimerReleasesIdleResources(t *testing.T) {
	manager := newTestLPResourceManager(t, 20*time.Millisecond)
	defer manager.Close()

	decoder, err := lilliput.NewDecoder(makeTestJPEG(t))
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}
	options := &lilliput.ImageOptions{
		FileType:              ".webp",
		Width:                 8,
		Height:                4,
		ResizeMethod:          lilliput.ImageOpsFit,
		DisableAnimatedOutput: true,
		EncodeTimeout:         time.Second,
	}
	result, err := manager.Transform(decoder, options, 16)
	decoder.Close()
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if len(result) == 0 {
		t.Fatal("Transform() returned an empty result")
	}

	waitForResourceState(t, manager, func() bool {
		return manager.imageOps == nil && manager.resizeBuffer == nil
	})
}

func TestLPResourceManagerCloseIsReusable(t *testing.T) {
	manager := newTestLPResourceManager(t, time.Hour)
	manager.mu.Lock()
	manager.ensureCapacityLocked(8)
	manager.recordSuccessfulTransformLocked(8, time.Now())
	manager.mu.Unlock()

	manager.Close()
	manager.Close()
	if manager.imageOps != nil || manager.resizeBuffer != nil || manager.idleTimer != nil {
		t.Fatal("Close should stop the timer and release all resources")
	}

	manager.mu.Lock()
	manager.ensureCapacityLocked(4)
	manager.recordSuccessfulTransformLocked(4, time.Now())
	manager.mu.Unlock()
	defer manager.Close()
	if manager.imageOpsMaxSize != 4 || manager.imageOps == nil {
		t.Fatal("manager should allocate resources after Close")
	}
}

func TestLPResourceManagerCloseInvalidatesStaleTimerCallback(t *testing.T) {
	manager := newTestLPResourceManager(t, time.Hour)
	manager.mu.Lock()
	manager.ensureCapacityLocked(8)
	manager.recordSuccessfulTransformLocked(8, time.Now())
	oldGeneration := manager.timerGeneration
	manager.mu.Unlock()

	manager.Close()
	manager.mu.Lock()
	manager.ensureCapacityLocked(4)
	manager.recordSuccessfulTransformLocked(4, time.Now())
	manager.mu.Unlock()

	manager.onRetentionTimer(oldGeneration)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.imageOps == nil || manager.imageOpsMaxSize != 4 {
		t.Fatal("stale callback should not release resources allocated after Close")
	}
}

func waitForResourceState(t *testing.T, manager *LPResourceManager, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		manager.mu.Lock()
		matched := condition()
		manager.mu.Unlock()
		if matched {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("resource manager did not reach expected state before timeout")
}

func newTestLPResourceManager(t *testing.T, retentionPeriod time.Duration) *LPResourceManager {
	t.Helper()
	manager, err := newLPResourceManagerWithRetentionPeriod(testTelemetry(t), retentionPeriod)
	if err != nil {
		t.Fatalf("newLPResourceManagerWithRetentionPeriod() error = %v", err)
	}
	return manager
}

func testTelemetry(t *testing.T) *telemetry.TelemetrySvc {
	t.Helper()
	t.Setenv("OTEL_ENABLED", "false")
	telemetryService, err := telemetry.NewTelemetrySvc(context.Background())
	if err != nil {
		t.Fatalf("NewTelemetrySvc() error = %v", err)
	}
	return telemetryService
}

func makeTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 20), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, nil); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return encoded.Bytes()
}
