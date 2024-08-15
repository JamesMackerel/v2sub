package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

func RequestSubscription(u string, proxyStr string) (string, error) {
	client := &http.Client{}

	// 解析代理服务器的字符串地址
	var proxyURL *url.URL
	if proxyStr != "" {
		var err error
		proxyURL, err = url.Parse(proxyStr)
		if err != nil {
			return "", fmt.Errorf("invalid proxy URL: %v", err)
		}
	}

	// 如果提供了代理URL，则设置HTTP客户端的代理
	if proxyURL != nil {
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err // 如果请求创建失败，返回错误
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err // 如果请求发送失败，返回错误
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned non-200 status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err // 如果读取响应体失败，返回错误
	}

	return string(body), nil
}

func ConvertSubscription(content string) string {
	var processedURLs []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if len(line) == 0 {
			continue // 跳过空行
		}

		// 找到 # 的位置
		slashIndex := strings.Index(line, "#")
		if slashIndex == -1 {
			processedURLs = append(processedURLs, line)
			continue
		}

		// 把 # 后面的内容 urlencode 一遍
		commentPart := line[slashIndex:]
		result := line[:slashIndex] + "#" + url.QueryEscape(commentPart)
		processedURLs = append(processedURLs, result)
	}

	return strings.Join(processedURLs, "\n")
}

func main() {
	// read CLI args
	subUrl := flag.String("subUrl", "", "Subscription URL")
	proxyUrl := flag.String("proxyUrl", "", "Proxy URL")
	listenAddr := flag.String("listen", "127.0.0.1:18888", "HTTP listen address:port")
	flag.Parse()

	if *subUrl == "" || *proxyUrl == "" {
		fmt.Println("Error: Both -subUrl and -proxyUrl are required")
		flag.PrintDefaults()
		return
	}

	var e = echo.New()
	e.GET("/", func(c echo.Context) error {
		fmt.Println("start get")
		subscription, err := RequestSubscription(*subUrl, *proxyUrl)
		if err != nil {
			fmt.Printf("error get sub: %v", err)
			return c.String(http.StatusInternalServerError, err.Error())
		}

		// url safe base64 needs padding completion
		equalsCount := 4 - (len(subscription) % 4)
		subscription += strings.Repeat("=", equalsCount)

		decodeString, err := base64.URLEncoding.DecodeString(subscription)
		if err != nil {
			fmt.Printf("failed to decode: %v", err)
			return c.String(http.StatusInternalServerError, err.Error())
		}

		fmt.Printf("got origin sub: %s", subscription)

		return c.String(http.StatusOK, ConvertSubscription(string(decodeString)))
	})

	fmt.Printf("Starting server on %s\n", *listenAddr)
	e.Logger.Fatal(e.Start(*listenAddr))
}
