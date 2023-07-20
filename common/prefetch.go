package common

import (
	"net/http"
	"path/filepath"
)

func GetCacheKey(req *http.Request) string {
	return req.URL.String()
}

func IsRequestCachable(req *http.Request) bool {
	cacheable := req.Method == "GET" && req.Header.Get("range") == ""
	if !cacheable {
		return false
	}
	ext := filepath.Ext(req.URL.Path)
	return ext == ".js" || ext == ".css"
}
