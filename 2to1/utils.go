package http2to1

import "golang.org/x/net/http2/hpack"

func getAuthorityFromHeaders(headers []hpack.HeaderField) string {
	for _, h := range headers {
		if h.Name == ":authority" {
			return h.Value
		}
	}
	return ""
}

func getValueByKeyFromHeaders(headers []hpack.HeaderField, key string) string {
	for _, h := range headers {
		if h.Name == key {
			return h.Value
		}
	}
	return ""
}
