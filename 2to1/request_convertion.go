package http2to1

import "golang.org/x/net/http2/hpack"

func h2HeadersToH1Headers(headers []hpack.HeaderField) []string {
	h1Headers := make([]string, len(headers))
	for i, h := range headers {
		h1Headers[i] = h.Name + ": " + h.Value
	}
	return h1Headers
}
