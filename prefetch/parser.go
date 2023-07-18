package prefetch

import "net/http"

func ExtractResources(resp *http.Response) []string {
	return []string{
		"https://pss.bdstatic.com/static/superman/js/lib/jquery-1-edb203c114.10.2.js",
	}
}
