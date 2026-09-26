# syntax=docker/dockerfile:1

# ---------- build stage ----------
FROM golang:1.22-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/waveguide-server ./cmd/server

# ---------- test stage ----------
# Run the automated suite inside the container with:
#   docker build --target test -t waveguide-test .
# (a non-zero test result fails the build)
FROM build AS test
RUN go test ./...

# ---------- runtime stage ----------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/waveguide-server /waveguide-server
ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/waveguide-server"]
