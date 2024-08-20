package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
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

func containsHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

type Config struct {
	SubUrl     string `json:"subUrl" yaml:"subUrl" toml:"subUrl"`
	ProxyUrl   string `json:"proxyUrl" yaml:"proxyUrl" toml:"proxyUrl"`
	ListenAddr string `json:"listenAddr" yaml:"listenAddr" toml:"listenAddr"`
}

func loadConfig(filePath string, config *Config) error {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	switch filepath.Ext(filePath) {
	case ".json":
		err = json.Unmarshal(file, config)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(file, config)
	case ".toml":
		err = toml.Unmarshal(file, config)
	default:
		return fmt.Errorf("unsupported file format")
	}
	return err
}

func prepareConfig() (*Config, error) {
	// Define CLI arguments
	subUrl := flag.String("subUrl", "", "Subscription URL")
	proxyUrl := flag.String("proxyUrl", "", "Proxy URL")
	listenAddr := flag.String("listen", "127.0.0.1:18888", "HTTP listen address:port")
	configPath := flag.String("config", "v2sub-conf.yml", "Path to the config file")
	flag.StringVar(configPath, "c", "v2sub-conf.yml", "Path to the config file (shorthand)")
	flag.Parse()

	// Check if -h or --help is passed
	if containsHelpFlag(os.Args) {
		flag.Usage()
		return nil, fmt.Errorf("help requested")
	}

	// Load config from file
	var config Config
	if err := loadConfig(*configPath, &config); err != nil {
		fmt.Println("Error loading config file:", err)
		flag.Usage()
		return nil, err
	}

	// Override config with CLI args if provided
	if *subUrl != "" {
		config.SubUrl = *subUrl
	}
	if *proxyUrl != "" {
		config.ProxyUrl = *proxyUrl
	}
	if *listenAddr != "" {
		config.ListenAddr = *listenAddr
	}

	// Validate required arguments
	if config.SubUrl == "" || config.ProxyUrl == "" {
		fmt.Println("Error: Both subUrl and proxyUrl are required")
		flag.PrintDefaults()
		return nil, fmt.Errorf("missing required arguments")
	}

	return &config, nil
}

func main() {
	config, err := prepareConfig()
	if err != nil {
		panic(err)
	}

	var e = echo.New()
	e.GET("/", func(c echo.Context) error {
		fmt.Println("start get")
		subscription, err := RequestSubscription(config.SubUrl, config.ProxyUrl)
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

	fmt.Printf("Starting server on %s\n", config.ListenAddr)
	e.Logger.Fatal(e.Start(config.ListenAddr))
}
