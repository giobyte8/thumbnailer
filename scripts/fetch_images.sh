#!/bin/bash
# Fetch some test images to use during development
# Usage: ./fetch_images.sh

SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
CALLER_DIR="$(pwd)"
cd "$SCRIPT_DIR"

# Load .env file if it exists
if [ -f "../.env" ]; then
    source ../.env
fi

DOWNLOAD_DST=$(realpath "$DIR_ORIGINALS_ROOT")
if [ ! -d "$DOWNLOAD_DST" ]; then
    echo "Directory does not exist: $DOWNLOAD_DST"
    exit 1
fi

# Clean up destination directory
rm "$DOWNLOAD_DST"/*

galleries=(
    "https://500px.com/p/kid_of_ozz/galleries/places"
    #"https://500px.com/p/kid_of_ozz/galleries/endless-summer-x-kidofozz"
    #"https://500px.com/p/DmitriySoloduhin/galleries/my-lego-photos"
)

# Download each gallery
echo "Downloading images into: $DOWNLOAD_DST"
for gallery in "${galleries[@]}"; do
    echo
    echo "Downloading gallery: $gallery"
    gallery-dl -D "$DOWNLOAD_DST" "$gallery"
done

cd "$CALLER_DIR"