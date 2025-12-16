# Development Setup

## Setting Up the Environment
Copy the `template.env` file to `.env` and update with your own values:
```bash
cp template.env .env && vim .env
# Enter appropriate values for environment variables.

# Install Go dependencies
go mod tidy
```

## Running the Project
To run the project in development mode, use the following command:
```bash
go run ./cmd/thumbnailer
```

This will start the Thumbnailer service and connect it to the RabbitMQ server
specified in your `.env` file.

### Notes
- Ensure RabbitMQ is running and accessible before starting the service.
- Logs will be printed to the console for debugging purposes.


## Release Process

When a new version is ready for release, follow below instructions to build
and publish a new docker image.

> NOTE: Make sure to prepare docker builder to build
[multi-arch images](https://giovanniaguirre.me/blog/docker_build_multiarch/)
before building new image version

### Build and push docker image

Pass the new version number as tag to the `build_release.sh` script, it will
take care of building the image, tag it with provided version and push it into
docker registry.

```shell
./docker/build_release.sh -t 1.0.0 -p
```

> Note: You can ommit the `-p` flag to prevent the push step and test the image locally first

#### Run image locally

Once the image is ready you can use a command like below to test it locally

```shell
docker run -it --rm giobyte8/thumbnailer:dev

# If you want to share a custom env file you can do:
docker run -it --rm \
  -v .env:/opt/thumbnailer/.env \
  giobyte8/thumbnailer:dev
```

