module github.com/zckevin/http2-mitm-proxy

go 1.20

require (
	github.com/google/martian/v3 v3.3.2
	github.com/nadoo/glider v0.16.3
	github.com/sagernet/sing v0.2.5
	github.com/sagernet/sing-box v1.2.7
	github.com/sagernet/sing-mux v0.1.0
	golang.org/x/exp v0.0.0-20230515195305-f3d0a9c9a5cc
	golang.org/x/net v0.11.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/miekg/dns v1.1.54 // indirect
	github.com/sagernet/sing-dns v0.1.5-0.20230415085626-111ecf799dfc // indirect
	github.com/sagernet/smux v0.0.0-20230312102458-337ec2a5af37 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
)

replace github.com/google/martian/v3 => /home/zc/PROJECTS/tcp/martian-origin

replace github.com/sagernet/sing-mux => /home/zc/PROJECTS/tcp/sing-mux

// replace golang.org/x/net => /home/zc/PROJECTS/tcp/net
// replace github.com/sagernet/sing => /home/zc/PROJECTS/tcp/sing
// replace github.com/sagernet/smux => /home/zc/PROJECTS/tcp/smux
