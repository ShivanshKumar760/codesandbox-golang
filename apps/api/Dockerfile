# ---- build stage ----
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app .

# ---- runtime stage ----
# No python here any more: the API never runs user code.
# ca-certificates is needed because we now call Google APIs over HTTPS.
FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 app
USER app
COPY --from=build /app /app
ENV PORT=8080
CMD ["/app"]
