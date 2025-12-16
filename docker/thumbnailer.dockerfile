FROM golang:1.24-bookworm AS builder
WORKDIR /opt/thumbnailer

# Install the C dependencies for lilliput
RUN apt-get update && apt-get install -y \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    libwebp-dev \
    # Clean up apt cache to reduce stage size
    && rm -rf /var/lib/apt/lists/*

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy only the relevant application source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application
#  The CGO_ENABLED=1 flag is crucial for Cgo-based packages like lilliput
#  The -o flag names the output binary
RUN CGO_ENABLED=1 GOOS=linux go build -o thumbnailer ./cmd/thumbnailer


# Final stage: Create a minimal image with the binary
FROM debian:bookworm-slim AS runtime
WORKDIR /opt/thumbnailer

# Install the runtime dependencies for lilliput
# We're installing the non-dev versions of the libraries
RUN apt-get update && apt-get install -y \
    libjpeg62-turbo \
    libpng16-16 \
    libtiff6 \
    libwebp7 \
    liblcms2-2 \
    libbz2-1.0 \
    # Clean up to keep the final image as small as possible
    && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /opt/thumbnailer/thumbnailer .
ENTRYPOINT ["./thumbnailer"]