# Thumbnailer

Thumbnails generator for image and video files.

## Service Overview

The Thumbnailer service listens for file discovery events from a RabbitMQ queue, processes the events to generate thumbnails for the discovered images, and stores the thumbnails in the output directory. It is designed to be modular and extensible, with clear abstractions for message consumption and thumbnail generation.

## How It Works

Below is a high-level diagram of the service's workflow:

```
+-------------------+       +---------------------------------------------+
|                   |       |                                             |
| RabbitMQ Queue    +------>+               Thumbnailer                   |
|                   |       |  +-------------------+       +------------+ |
+-------------------+       |  |                   |       |            | |
                            |  |     Consumer      +------>+ Thumbnails | |
                            |  |                   |       | Generator  | |
                            |  +-------------------+       +------------+ |
                            |                                             |
                            +---------------------------------------------+
                                                              |
                                                              v
                                                +-------------------------------+
                                                |                               |
                                                |   Output Directory (thumbs)   |
                                                |                               |
                                                +-------------------------------+
```

### Workflow Steps
1. **RabbitMQ Queue**: The service listens to a specific RabbitMQ queue for file discovery events.
2. **Consumer**: The `Consumer` component processes incoming messages and extracts relevant metadata about the discovered files.
3. **Thumbnails Generator**: The `Thumbnails Generator` component generates thumbnails for the images using the configured library (e.g., Lilliput).
4. **Output Directory**: The generated thumbnails are stored in the configured path