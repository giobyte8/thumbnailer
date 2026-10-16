package thumbsgen

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/discord/lilliput"
	"github.com/giobyte8/thumbnailer/internal/telemetry"
	"github.com/giobyte8/thumbnailer/internal/telemetry/metrics"
)

const (
	lpResourcesRetentionEnv     = "LP_RESOURCES_MEMORY_RETENTION_PERIOD"
	lpResourcesDefaultRetention = 5 * time.Minute
	lpResizeBufferSize          = 50 * 1024 * 1024
)

// LPResourceManager owns reusable Lilliput transform resources. It serializes
// transforms with timer-driven shrinking and release so resources are never
// replaced while in use. The timer evaluates fixed retention windows; each
// active window either shrinks underused capacity or retains it for another
// window, while a fully idle window releases all resources.
type LPResourceManager struct {
	// mu serializes transforms, timer callbacks, and resource cleanup.
	mu sync.Mutex

	telemetry *telemetry.TelemetrySvc

	// idleTimer evaluates the current retention window and is nil when no
	// resources are retained.
	idleTimer *time.Timer

	// timerGeneration invalidates callbacks that were already running at Stop.
	timerGeneration uint64

	// lastTransformTime marks the most recent successful transform for idle checks.
	lastTransformTime time.Time

	// imageOps is nil until a transform needs Lilliput resources.
	// This is the heaviest resource, it is created by Lilliput and its memory
	// usage depends on the maximum image size it needs to handle
	//
	// This instance is shared across requests make to transform an image,
	// if you need to process requests concurrently, consider using a pool
	// of ImageOps or creating a new LpResourceManager per worker to avoid
	// race conditions.
	//
	// Here is how it works:
	//   ImageOps memory sizing:
	//     - ImageOps stores processing pixels during transformations.
	//     - Each pixel requires 4 bytes.
	//     - Two framebuffers are used for double-buffering; animated inputs may need
	//       a third framebuffer.
	//     - NewImageOps(maxSize) allocates each framebuffer as maxSize x maxSize;
	//       this manager receives the larger source-image axis as maxSize.
	//
	//   So, required memory can be computed like:
	//    NewImageOps(maxDimension) = 4 bytes * maxDimension^2 * 2 buffers
	//                                      4 * maxDimension^2 * 2
	//                                      8 * maxDimension^2
	//
	//   Example memory requirements:
	//   Max Dimension  | Per buffer | 2 buffers
	//   8192           | ~256 MiB   | ~512 MiB
	//   16320          | ~1.02 GiB  | ~2.04 GiB
	//   See: https://deepwiki.com/discord/lilliput/8.2-batch-processing
	//
	// ImageOps.Clear() zeros pixel data for reuse but retains framebuffer storage.
	// ImageOps.Close() releases native resources; dropping the ImageOps reference
	//   makes its Go buffers eligible for garbage collection.
	//
	// This manager allocates only the required capacity, then shrinks or releases
	// it after retention checks.
	imageOps *lilliput.ImageOps

	// imageOpsMaxSize is the largest supported axis of imageOps.
	imageOpsMaxSize int

	// retentionPeriod controls idle release and capacity-evaluation windows.
	retentionPeriod time.Duration

	// periodMaxRequiredSize is the largest axis required during the current window.
	periodMaxRequiredSize int

	// resizeBuffer is allocated with imageOps and released when resources go idle.
	resizeBuffer []byte
}

// NewLPResourceManager creates a manager using the configured retention period.
// LP_RESOURCES_MEMORY_RETENTION_PERIOD is an optional positive integer in
// seconds; an unset or blank value defaults to five minutes.
func NewLPResourceManager(telemetryService *telemetry.TelemetrySvc) (*LPResourceManager, error) {
	retentionValue := strings.TrimSpace(os.Getenv(lpResourcesRetentionEnv))
	if retentionValue == "" {
		return newLPResourceManagerWithRetentionPeriod(telemetryService, lpResourcesDefaultRetention)
	}

	retentionSeconds, err := strconv.ParseInt(retentionValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", lpResourcesRetentionEnv, retentionValue, err)
	}
	if retentionSeconds <= 0 {
		return nil, fmt.Errorf("%s must be a positive integer: %d", lpResourcesRetentionEnv, retentionSeconds)
	}
	if retentionSeconds > math.MaxInt64/int64(time.Second) {
		return nil, fmt.Errorf("%s is too large: %d", lpResourcesRetentionEnv, retentionSeconds)
	}

	return newLPResourceManagerWithRetentionPeriod(telemetryService, time.Duration(retentionSeconds)*time.Second)
}

// newLPResourceManagerWithRetentionPeriod constructs a manager with an explicit
// duration. It supports focused tests and future callers that inject a policy
// instead of relying on process environment configuration.
func newLPResourceManagerWithRetentionPeriod(
	telemetryService *telemetry.TelemetrySvc,
	retentionPeriod time.Duration,
) (*LPResourceManager, error) {
	if retentionPeriod <= 0 {
		return nil, fmt.Errorf("resource retention period must be positive")
	}

	manager := &LPResourceManager{
		retentionPeriod: retentionPeriod,
		telemetry:       telemetryService,
	}
	manager.telemetry.Metrics().Gauge(metrics.LPResourcesImageOpsMaxSize, 0)
	return manager, nil
}

// Transform applies Lilliput options using resources sized for requiredSize.
// It serializes access, clears Lilliput's working pixels for reuse, and returns
// bytes independent of the manager's reusable output buffer.
func (m *LPResourceManager) Transform(
	decoder lilliput.Decoder,
	options *lilliput.ImageOptions,
	requiredSize int,
) ([]byte, error) {
	if requiredSize <= 0 {
		return nil, fmt.Errorf("required image dimension must be positive: %d", requiredSize)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureCapacityLocked(requiredSize)
	transformed, err := m.imageOps.Transform(decoder, options, m.resizeBuffer)
	m.imageOps.Clear()
	if err != nil {
		if m.lastTransformTime.IsZero() && m.idleTimer == nil {
			m.releaseResourcesLocked()
		}
		return nil, err
	}

	result := append([]byte(nil), transformed...)
	m.recordSuccessfulTransformLocked(requiredSize, time.Now())
	return result, nil
}

// Close stops retention callbacks and releases current resources. It is safe
// to call repeatedly; a later Transform can allocate resources again.
func (m *LPResourceManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopTimerLocked()
	m.releaseResourcesLocked()
}

// ensureCapacityLocked lazily allocates or expands resources to requiredSize.
// The mutex must be held by the caller.
func (m *LPResourceManager) ensureCapacityLocked(requiredSize int) {
	if m.imageOpsMaxSize < requiredSize {
		if m.imageOps != nil {
			m.imageOps.Close()
			m.imageOps = nil
		}
		m.imageOps = lilliput.NewImageOps(requiredSize)
		m.setImageOpsMaxSizeLocked(requiredSize)
		if requiredSize > m.periodMaxRequiredSize {
			m.periodMaxRequiredSize = requiredSize
		}
	}
	if m.resizeBuffer == nil {
		m.resizeBuffer = make([]byte, lpResizeBufferSize)
	}
}

// recordSuccessfulTransformLocked records use and starts the first fixed
// retention window. Later transforms update the window but do not move its timer.
func (m *LPResourceManager) recordSuccessfulTransformLocked(
	requiredSize int,
	transformTime time.Time,
) {
	m.lastTransformTime = transformTime
	if requiredSize > m.periodMaxRequiredSize {
		m.periodMaxRequiredSize = requiredSize
	}
	if m.idleTimer == nil {
		m.scheduleTimerLocked(m.retentionPeriod)
	}
}

// scheduleTimerLocked starts one callback for the next maintenance check.
// The mutex must be held by the caller.
func (m *LPResourceManager) scheduleTimerLocked(delay time.Duration) {
	m.timerGeneration++
	generation := m.timerGeneration
	m.idleTimer = time.AfterFunc(delay, func() {
		m.onRetentionTimer(generation)
	})
}

// onRetentionTimer ignores stale callbacks, then evaluates and rearms the
// retention window while holding the same lock used by Transform and Close.
func (m *LPResourceManager) onRetentionTimer(generation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if generation != m.timerGeneration {
		return
	}
	m.idleTimer = nil
	if m.imageOps == nil {
		return
	}

	nextDelay := m.maintainResourcesLocked(time.Now())
	if nextDelay > 0 && m.imageOps != nil {
		m.scheduleTimerLocked(nextDelay)
	}
}

// maintainResourcesLocked releases idle resources or shrinks capacity below
// 90% utilization, then clears the completed window's usage maximum. The mutex
// must be held by the caller; its return value is the next callback delay.
func (m *LPResourceManager) maintainResourcesLocked(now time.Time) time.Duration {
	if now.Sub(m.lastTransformTime) >= m.retentionPeriod {
		m.releaseResourcesLocked()
		return 0
	}

	if m.periodMaxRequiredSize > 0 && int64(m.periodMaxRequiredSize)*10 < int64(m.imageOpsMaxSize)*9 {
		m.replaceImageOpsLocked(m.periodMaxRequiredSize)
	}
	m.periodMaxRequiredSize = 0

	untilIdle := m.lastTransformTime.Add(m.retentionPeriod).Sub(now)
	if untilIdle < m.retentionPeriod {
		return untilIdle
	}
	return m.retentionPeriod
}

// replaceImageOpsLocked closes the old Lilliput instance before replacing it.
// The mutex must be held by the caller.
func (m *LPResourceManager) replaceImageOpsLocked(requiredSize int) {
	if m.imageOps != nil {
		m.imageOps.Close()
		m.imageOps = nil
	}

	m.imageOps = lilliput.NewImageOps(requiredSize)
	m.setImageOpsMaxSizeLocked(requiredSize)
}

// stopTimerLocked invalidates even a callback that already began running.
// The mutex must be held by the caller.
func (m *LPResourceManager) stopTimerLocked() {
	m.timerGeneration++
	if m.idleTimer != nil {
		m.idleTimer.Stop()
		m.idleTimer = nil
	}
}

// releaseResourcesLocked closes Lilliput resources and drops Go references so
// their backing buffers become eligible for garbage collection. The mutex must
// be held by the caller.
func (m *LPResourceManager) releaseResourcesLocked() {
	if m.imageOps != nil {
		m.imageOps.Close()
		m.imageOps = nil
	}
	m.setImageOpsMaxSizeLocked(0)
	m.resizeBuffer = nil
	m.periodMaxRequiredSize = 0
	m.lastTransformTime = time.Time{}
}

func (m *LPResourceManager) setImageOpsMaxSizeLocked(size int) {
	if m.imageOpsMaxSize == size {
		return
	}

	m.imageOpsMaxSize = size
	m.telemetry.Metrics().Gauge(metrics.LPResourcesImageOpsMaxSize, int64(size))
}
