package html_parser

/*
import (
	"fmt"
	"net/http"
)

func main() {
	target := "https://www.zhihu.com"
	req, _ := http.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("accept-encoding", "gzip, deflate, br")
	req.Header.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64; rv:78.0) Gecko/20100101 Firefox/78.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	s, err := GetHTMLHeadContent(resp)
	if err == nil {
		fmt.Println(111, string(s))
		if urls, err := ExtractResourcesInHead(resp); err == nil {
			fmt.Println(urls)
		}
	}
	fmt.Println(len(s), resp.Header.Get("content-encoding"), err)
}
*/
