default: start-client

pwd := `pwd`
log_level := "3"
cert := "--cert=./certs/cert.crt --key=./certs/cert.key"
server_addr := ""
project_name := "http2-mitm-proxy"

start-server:
  go run ./cmd/server --log-level=0

start-client:
  go run ./cmd/client --log-level={{log_level}} {{cert}} --server-addr={{server_addr}}

start-server-v:
  GODEBUG=http2debug=2 go run ./cmd/server --log-level={{log_level}} 

start-server-vv:
  go run ./cmd/server --log-level={{log_level}} --debug

start-server-vvv:
  GODEBUG=http2debug=2 go run ./cmd/server --log-level={{log_level}} --debug

start-client-debug:
  go run ./cmd/client --log-level={{log_level}} {{cert}} --server-addr={{server_addr}} --debug

start-prefetch-static-server:
  ran -b 127.0.0.1 -tls-port=2001 {{cert}} -r ./tmp

# build GLIBC compatible cgo binary in docker for debian 11 bullseye
# first build brotli into tcp/google-brotli
# ref to https://archive.is/QjDml
build-server-release:
  docker run --rm \
    -v {{pwd}}/../:/tcp \
    -v $GOPATH:/go \
    -w /tcp/{{project_name}} \
    --env GOFLAGS="-buildvcs=false" \
    --env CGO_ENABLED=1 \
    --env CGO_CFLAGS="-I/tcp/google-brotli/installed/include" \
    --env CGO_LDFLAGS="-L/tcp/google-brotli/installed/lib64" \
    golang:1.20.6-bullseye \
    go build ./cmd/server
