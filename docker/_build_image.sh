#!/usr/bin/env bash
# Builds and optionally pushes a new version of a docker image
set -e

SCRIPT_PATH="$(cd -- "$(dirname "$0")" >/dev/null 2>&1; pwd -P)"
APP_ROOT="$(dirname "$SCRIPT_PATH")"

PUSH_IMAGE=false
SKIP_SMOKE=false

function usage {
  echo "Usage: $0 [-h] [-p] [-s] image tag dockerfile"
  echo "  -h              Show this help message"
  echo "  -p              Push the image after build"
  echo "  -s              Skip smoke test after build"
  echo "  image           Name for the docker image"
  echo "  tag             Tag for the docker image"
  echo "  dockerfile      Path to the Dockerfile to build"
}

# Parse arguments and options regardless its position
# Ref: https://stackoverflow.com/a/63421397/3211029
args=()
while [ $OPTIND -le "$#" ]
do
  if getopts "hps" flag
  then
    case $flag in
      h)
        usage
        exit 0
        ;;
      p)
        PUSH_IMAGE=true
        ;;
      s)
        SKIP_SMOKE=true
        ;;
    esac
  else
    args+=("${!OPTIND}")
    ((OPTIND++))
  fi
done

# Read positional arguments
IMAGE_NAME="${args[0]}"
IMAGE_TAG="${args[1]}"
DOCKERFILE="${args[2]}"

# Validate image name is provided
if [ -z "$IMAGE_NAME" ]; then
  echo "Error: image name is required" >&2
  exit 1
fi

# Validate image tag is provided
if [ -z "$IMAGE_TAG" ]; then
  echo "Error: image tag is required" >&2
  exit 1
fi

# Validate dockerfile is provided
if [ -z "$DOCKERFILE" ]; then
  echo "Error: dockerfile is required" >&2
  exit 1
fi

# Validate dockerfile existense
if [ ! -f "$DOCKERFILE" ]; then
  echo "Error: dockerfile does not exist" >&2
  exit 1
fi

IMAGE_REF="${IMAGE_NAME}:${IMAGE_TAG}"

echo "Building image: ${IMAGE_REF}"
echo "    Dockerfile: ${DOCKERFILE}"
echo "    Skip smoke: ${SKIP_SMOKE}"
echo "    Push image: ${PUSH_IMAGE}"
echo

echo "Building local image: ${IMAGE_REF}"
docker build          \
  -t "${IMAGE_REF}"   \
  -f "${DOCKERFILE}"  \
  "${APP_ROOT}"

if [ "$SKIP_SMOKE" = false ]; then
  echo
  echo "Running smoke check for: ${IMAGE_REF}"
  ./image_verify.sh "$IMAGE_REF"
fi

if [ "$PUSH_IMAGE" = true ]; then
  echo
  echo "Publishing multi-arch image: ${IMAGE_REF}"

  # Multi arch build and push
  docker buildx build                   \
    --platform linux/amd64,linux/arm64  \
    -t "${IMAGE_REF}"   \
    -f "${DOCKERFILE}"                  \
    --push                              \
    "${APP_ROOT}"

  echo
  echo "Image built and released to docker registry: ${IMAGE_REF}"
else
  echo
  echo "Image ready for local usage: ${IMAGE_REF}"
fi
