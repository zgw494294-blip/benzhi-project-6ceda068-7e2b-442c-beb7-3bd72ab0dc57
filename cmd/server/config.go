package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	address         string
	dataDirectory   string
	selfCheck       bool
	shutdownTimeout time.Duration
}

func parseConfig(arguments []string) (config, error) {
	set := flag.NewFlagSet("astronomy-release-governance", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	address := set.String("addr", "", "HTTP 监听地址，必须为回环地址")
	dataDirectory := set.String("data-dir", "", "事件日志与快照目录")
	selfCheck := set.Bool("selfcheck", false, "启动真实监听并执行有界 HTTP 自检后退出")
	shutdownTimeout := set.Duration("shutdown-timeout", 5*time.Second, "优雅关闭期限")
	if err := set.Parse(arguments); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("不接受位置参数：%s", strings.Join(set.Args(), " "))
	}
	resolvedAddress := strings.TrimSpace(*address)
	if resolvedAddress == "" {
		portValue := strings.TrimSpace(os.Getenv("PORT"))
		if portValue != "" {
			port, err := strconv.Atoi(portValue)
			if err != nil || port < 1 || port > 65535 {
				return config{}, fmt.Errorf("PORT 必须是 1 到 65535 之间的端口号")
			}
			resolvedAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		} else {
			resolvedAddress = defaultAddress
		}
	}
	if err := validateAddress(resolvedAddress); err != nil {
		return config{}, err
	}
	if *shutdownTimeout <= 0 || *shutdownTimeout > time.Minute {
		return config{}, fmt.Errorf("shutdown-timeout 必须大于零且不超过一分钟")
	}
	return config{
		address: resolvedAddress, dataDirectory: strings.TrimSpace(*dataDirectory),
		selfCheck: *selfCheck, shutdownTimeout: *shutdownTimeout,
	}, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("addr 必须使用 host:port 格式：%w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("addr 端口必须在 1 到 65535 之间")
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("addr 必须绑定回环地址，拒绝 %q", host)
	}
	return nil
}
