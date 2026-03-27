# internal/thumbs_gen refactor spec

## Goal
Refactor `internal/thumbs_gen` to separate concerns and improve testability,
while preserving current behavior from service perspective:

- `ThumbnailsService` still calls a single `ThumbsGenerator`.
- `RoutedThumbsGenerator` still decides by input extension.
- Unsupported extensions are still discarded with warning and **no error**.
- Final thumbnail output stays `*.webp` with existing naming format.

## Current baseline to preserve

- Routing:
  - Video: `.mp4`, `.mov`
  - Image: `.jpg`, `.jpeg`, `.png`, `.heic`
- `ThumbsGenerator` public contract remains:

```go
type ThumbsGenerator interface {
    Generate(ctx context.Context, meta ThumbnailMeta) error
}
```

## Scope

### In scope

1. Introduce frame extraction package for video-to-image extraction.
2. Introduce format conversion package for HEIC-to-JPEG conversion.
3. Refactor generators to consume those abstractions via dependency injection.
4. Keep routing and service orchestration behavior unchanged.
5. Add/adjust tests for new packages and refactored behavior.

### Non-goals

- No changes to AMQP consumer logic.
- No changes to environment-variable schema.
- No changes to output file extension/quality defaults.
- No service-layer branching by media type.

## Target architecture

### 1) `internal/thumbs_gen/frame_extractor`

Purpose: extract one representative JPG frame from video files.

#### Interface

```go
package frameextractor

import "context"

type VideoFrameExtractor interface {
    Extract(ctx context.Context, fromAbsPath string, intoAbsPath string) error
}
```

#### Implementation

- `FFmpegFrameExtractor` (uses `os/exec`)
- Validate input extension (initially `.mp4`, `.mov`)
- Validate output extension (initially `.jpg`)
- Return actionable errors for:
  - unsupported input/output extension
  - ffmpeg binary not found
  - command failure (include trimmed combined output)
- Keep extension maps centralized for easy updates.

#### ffmpeg command

Baseline command (match current preference):

```bash
ffmpeg -y -i input.mov -vframes 1 -vf format=yuv420p -q:v 2 output.jpg
```

Optional tuning:

- `-ss 00:00:01` can be added before `-i` if first-frame extraction produces
  black/blank frames for specific videos.

### 2) `internal/thumbs_gen/format_converter`

Purpose: convert HEIC originals into JPEG before image thumbnail generation.

#### Interface

```go
package formatconverter

import "context"

type ImageFormatConverter interface {
    HEICToJPEG(ctx context.Context, fromAbsPath string, intoAbsPath string) error
}
```

#### Implementation

- `HeifConvertFormatConverter` (uses `os/exec`)
- Validate input extension (initially `.heic`)
- Validate output extension (initially `.jpg`)
- Return actionable errors for:
  - unsupported input/output extension
  - `heif-convert` binary not found
  - command failure (include trimmed combined output)
- Keep extension maps centralized for easy updates.

### 3) `internal/thumbs_gen` generator refactor

#### `LilliputThumbsGenerator`

- Inject `ImageFormatConverter` dependency (interface, not concrete type).
- For `.heic` input, convert to temporary `.jpg` inside thumbnail directory.
- Use converted file as Lilliput input.
- Clean up temporary converted file using `defer` best effort cleanup.
- For non-HEIC, keep current direct flow.

#### `FFmpegThumbsGenerator`

- Inject dependencies:
  - `VideoFrameExtractor`
  - image thumbnail generator dependency via `ThumbsGenerator`
- New flow:
  1. Extract frame into temporary `.jpg` in thumbnail directory.
  2. Generate final `.webp` thumbnails from that JPG by delegating to injected
     image generator.
  3. Delete temporary JPG regardless of success/failure (`defer`).
- Important: avoid recursive delegation through `RoutedThumbsGenerator`.
  Delegate specifically to image generator implementation.

#### `RoutedThumbsGenerator`

- Keep as the single extension dispatch layer.
- Keep behavior for unsupported extensions:
  - `slog.Warn(...)`
  - return `nil`

## File and naming rules

- Keep existing thumbnail naming untouched (`<base>_<width>px.webp`).
- Temporary files:
  - should live in `meta.ThumbFileAbsDir`
  - should use deterministic prefix + unique suffix (safe for concurrent jobs)
  - should be removed on both success and failure

## Error and cancellation policy

- Respect `context.Context` cancellation in all subprocess execution paths.
- Wrap errors with `%w` and include high-signal metadata (file path, width,
  extension).
- Do not swallow internal execution errors (except unsupported extension path
  at routed layer, which intentionally returns `nil`).

## Logging and metrics policy

- Keep structured logs via `slog`.
- Keep warning log for unsupported extension at routing layer.
- Preserve thumbnail-created metric behavior (increment when final thumbnail is
  written successfully).
- Avoid duplicate metric increments when delegating across generators.

## Testing strategy

### Unit tests (fast, deterministic)

- `frame_extractor`:
  - extension validation
  - command args creation
  - binary-not-found and command-failure error shaping
- `format_converter`:
  - extension validation
  - command args creation
  - binary-not-found and command-failure error shaping
- refactored generators:
  - delegation wiring and call sequencing with test doubles
  - temporary file cleanup on success/failure
  - unsupported extension behavior remains unchanged in routed generator

### Integration tests (tooling-dependent, skip when unavailable)

- extractor with sample `.mp4` / `.mov` -> creates valid `.jpg`
- converter with sample `.heic` -> creates valid `.jpg`
- keep existing integration tests passing

## Rollout plan (Copilot execution phases)

### Phase 1

- Add `frame_extractor` package (interface + ffmpeg implementation + unit tests).
- Add `format_converter` package (interface + heif implementation + unit tests).

### Phase 2

- Refactor `LilliputThumbsGenerator` to consume `ImageFormatConverter` via DI.
- Keep current behavior and existing tests green.

### Phase 3

- Refactor `FFmpegThumbsGenerator` to:
  - use `VideoFrameExtractor`
  - delegate image thumbnail generation to injected image generator
  - manage temporary frame lifecycle

### Phase 4

- Wire constructors in `cmd/thumbnailer/main.go`.
- Ensure routed generator mappings stay explicit and easy to change.

### Phase 5

- Update/add tests impacted by dependency injection.
- Run focused tests first, then broader package tests.

## Definition of done

- Build succeeds.
- Existing `internal/thumbs_gen` tests still pass.
- New unit tests for `frame_extractor` and `format_converter` pass.
- Unsupported extension behavior unchanged (warn + nil).
- No temporary intermediate files remain after processing.
- Service wiring still uses `RoutedThumbsGenerator` as entrypoint.

## Copilot execution contract (in-file instructions)

If Copilot is implementing this refactor and this file is in context, treat the
rules below as **mandatory**.

### Operating mode

1. Work in **one phase at a time** (`Phase 1`..`Phase 5`).
2. Do not start the next phase until current phase is complete.
3. Keep changes minimal and scoped to the active phase.

### Mandatory implementation rules

1. Preserve current external behavior unless this spec explicitly changes it.
2. Keep `ThumbnailsService` using one `ThumbsGenerator` dependency.
3. Keep unsupported extension behavior unchanged at routed layer:
  - warning log
  - return `nil`
4. Use constructor injection and interfaces for new dependencies.
5. Do not introduce global mutable state.
6. Keep output thumbnail format and naming unchanged.

### File edit constraints

1. Edit only files required by the active phase.
2. Do not perform unrelated refactors or style-only rewrites.
3. Keep package boundaries aligned with this spec:
  - `internal/thumbs_gen/frame_extractor`
  - `internal/thumbs_gen/format_converter`

### Testing and validation requirements

1. Add or update tests for every behavior changed in the active phase.
2. Run focused tests for changed packages before broader test runs.
3. If integration tools are unavailable (`ffmpeg`, `heif-convert`), skip with
  explicit reason instead of failing silently.

### Required delivery format after each phase

After finishing a phase, report using this structure:

1. `Phase completed`: which phase was implemented.
2. `Files changed`: concise list.
3. `Behavior changes`: what changed (if any).
4. `Tests`: what ran, pass/fail/skip summary.
5. `Next phase`: ready/not ready and why.

### Stop conditions

Stop and ask for guidance if any of the following occurs:

1. A required behavior conflicts with this spec.
2. A dependency/API decision is ambiguous and changes architecture.
3. Tests fail for reasons unrelated to active-phase changes.
