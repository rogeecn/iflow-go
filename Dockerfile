FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/iflow-go .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/iflow-go /usr/local/bin/iflow-go

EXPOSE 28000

ENTRYPOINT ["/usr/local/bin/iflow-go"]
CMD ["serve", "--host", "0.0.0.0", "--port", "28000"]
