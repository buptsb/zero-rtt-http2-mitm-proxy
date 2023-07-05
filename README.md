# http2-mitm-proxy
HTTP MITM proxy over HTTP2 overy smux multiplexer, zero rtt, no cwnd slow start

## Definitions
- e2e rtt: end to end round trip time, from client side(your desktop pc) to origin server(e.g. cloudflare.com)
  - for oversea tcp connection, like EU-US or ASIA-US, it's usually 100ms+ or even 200ms+
- server side rtt: rtt from proxy server(maybe a VPS) to origin server(cloudflare.com again)
  - assume your proxy server's colo is near to origin server geographically, it's usually 10ms or even sub ms

## Why
- http agent should near to origin server
  - [tls handshake] + [TCP cwnd bump up] is done on remote proxy server
- zero rtt
  - over a mitm proxy, which forward data (HTTP2 frames) only
- no cwnd slow start
  - N:1, mux all connections over a single TCP conn

## Features
- very low latency (theoretically)
  - 0 end to end handshake + HTTP request(1 rtt) + server side standard HTTPS request = 1 * e2e_rtt + 3 * server_side_rtt
  - compared to a standard HTTPS over a TCP proxy:
    - TCP handshake(1 rtt) + TLS 1.2 handshake(2 rtts) + HTTP request(1 rtt) = 4 * e2e_rtt
- HTTP/2 on local proxy side instead of HTTP/1.1
  - in case Chrome has 6 conns per proxy host limitation
- HTTP 2to1 translation
  - support non-HTTP2 websites 
