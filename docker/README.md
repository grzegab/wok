# Project building instruction

This directory contains instructions for building the Docker image for the project.

## Building the image

To build the Docker image, run the following command from the **project root directory**:

```bash
docker build -t card-landing -f docker/Dockerfile .
```

## Running the container

After building the image, you can run the application with:

```bash
docker run -p 80:80 card-landing
```

## Technical details

The `Dockerfile` is divided into two stages:
1. **Builder stage:** Uses the `golang:1.26-alpine` image to compile the source code into a static Linux binary.
2. **Final stage:** Uses the lightweight `alpine:latest` image, to which the ready binary and necessary resources (HTML templates, static files) are copied.

This makes the final image small and secure.
