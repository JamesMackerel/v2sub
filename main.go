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
	"go.uber.org/zap"
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
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

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
	VerboseLog bool   `json:"verboseLog" yaml:"verboseLog" toml:"verboseLog"`
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

func CheckAnyParamExists(params []string) bool {
	for _, arg := range os.Args[1:] { // Skip the first argument (program name)
		for _, param := range params {
			if arg == param {
				return true
			}
		}
	}

	// Return false if none of the parameters are found
	return false
}

func prepareConfig() (*Config, error) {
	// Define CLI arguments
	subUrl := flag.String("subUrl", "", "Subscription URL")
	proxyUrl := flag.String("proxyUrl", "", "Proxy URL")
	listenAddr := flag.String("listen", "127.0.0.1:18888", "HTTP listen address:port")
	configPath := flag.String("config", "v2sub-conf.yml", "Path to the config file, if use default value, must be present at the last argument")
	verboseLog := flag.Bool("verboseLog", false, "Print verbose log")
	flag.StringVar(configPath, "c", "v2sub-conf.yml", "Path to the config file (shorthand), if use default value, must be present at the last argument")
	flag.Parse()

	// Check if -h or --help is passed
	if containsHelpFlag(os.Args) {
		flag.Usage()
		return nil, fmt.Errorf("help requested")
	}

	var config Config

	// Load config from file
	if CheckAnyParamExists([]string{"-config", "-c"}) {
		if err := loadConfig(*configPath, &config); err != nil {
			flag.Usage()
			return nil, err
		}
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
	if CheckAnyParamExists([]string{"-verboseLog"}) {
		config.VerboseLog = *verboseLog
	}

	// Validate required arguments
	if config.SubUrl == "" || config.ProxyUrl == "" {
		flag.PrintDefaults()
		return nil, fmt.Errorf("missing required arguments")
	}

	return &config, nil
}

func buildLogger() (*zap.Logger, *zap.SugaredLogger) {
	// Initialize zap logger
	zapConfig := zap.NewDevelopmentConfig()
	_logger, err := zapConfig.Build()
	_slog := _logger.Sugar()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	return _logger, _slog
}

var logger, slog = buildLogger()

func main() {
	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)

	config, err := prepareConfig()
	if err != nil {
		slog.Fatalf("failed to prepare config: %v", err)
	}

	var e = echo.New()
	e.GET("/", func(c echo.Context) error {
		logger.Info("start get request")
		subscription, err := RequestSubscription(config.SubUrl, config.ProxyUrl)
		if err != nil {
			logger.Error("error getting subscription", zap.Error(err))
			return c.String(http.StatusInternalServerError, err.Error())
		}

		// url safe base64 needs padding completion
		equalsCount := 4 - (len(subscription) % 4)
		subscription += strings.Repeat("=", equalsCount)

		decodeString, err := base64.URLEncoding.DecodeString(subscription)
		if err != nil {
			logger.Error("failed to decode subscription", zap.Error(err))
			return c.String(http.StatusInternalServerError, err.Error())
		}

		if config.VerboseLog {
			fmt.Println("original subscription:")
			fmt.Println(string(decodeString))
		}

		return c.String(http.StatusOK, ConvertSubscription(string(decodeString)))
	})

	logger.Info("starting server", zap.String("listenAddr", config.ListenAddr))
	slog.Fatal("%v", e.Start(config.ListenAddr))
}
