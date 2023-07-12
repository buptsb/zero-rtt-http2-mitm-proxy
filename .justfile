default: start-client

pwd := `pwd`
log_level := "3"
cert := "--cert=./certs/cert.crt --key=./certs/cert.key"
server_addr := ""

start-server:
  go run ./cmd/server --log-level={{log_level}}

start-client:
  go run ./cmd/client --log-level={{log_level}} {{cert}} --server-addr={{server_addr}}

start-server-debug:
  go run ./cmd/server --log-level={{log_level}} --debug

start-client-debug:
  go run ./cmd/client --log-level={{log_level}} {{cert}} --server-addr={{server_addr}} --debug