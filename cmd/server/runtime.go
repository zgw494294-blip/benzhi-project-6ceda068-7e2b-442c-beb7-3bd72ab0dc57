package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/httpapi"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func run(cfg config) error {
	dataDirectory, cleanup, err := resolveDataDirectory(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	repository, err := persistence.Open(dataDirectory)
	if err != nil {
		return err
	}
	defer repository.Close()
	service := archive.NewService(repository, time.Now, &archive.RandomIDGenerator{})
	api := httpapi.New(service)
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("监听 %s 失败：%w", cfg.address, err)
	}
	server := &http.Server{
		Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	serveErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrors <- err
			return
		}
		serveErrors <- nil
	}()
	if cfg.selfCheck {
		return runBoundedSelfCheck(server, listener, serveErrors, cfg.shutdownTimeout)
	}
	log.Printf("天文观测数据科研释放治理服务已监听 http://%s，数据目录 %s", listener.Addr().String(), dataDirectory)
	return waitForShutdown(server, serveErrors, cfg.shutdownTimeout)
}

func resolveDataDirectory(cfg config) (string, func(), error) {
	if cfg.dataDirectory != "" {
		absolute, err := filepath.Abs(cfg.dataDirectory)
		if err != nil {
			return "", func() {}, fmt.Errorf("解析数据目录失败：%w", err)
		}
		return absolute, func() {}, nil
	}
	if cfg.selfCheck {
		directory, err := os.MkdirTemp("", "astronomy-release-selfcheck-*")
		if err != nil {
			return "", func() {}, fmt.Errorf("创建自检数据目录失败：%w", err)
		}
		return directory, func() { _ = os.RemoveAll(directory) }, nil
	}
	absolute, err := filepath.Abs("data")
	if err != nil {
		return "", func() {}, fmt.Errorf("解析默认数据目录失败：%w", err)
	}
	return absolute, func() {}, nil
}

func runBoundedSelfCheck(server *http.Server, listener net.Listener, serveErrors <-chan error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	baseURL := "http://" + listener.Addr().String()
	checkErr := httpapi.RunSelfCheck(ctx, baseURL)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	serveErr := <-serveErrors
	if checkErr != nil {
		return checkErr
	}
	if shutdownErr != nil {
		return fmt.Errorf("自检后关闭服务失败：%w", shutdownErr)
	}
	if serveErr != nil {
		return fmt.Errorf("自检服务异常退出：%w", serveErr)
	}
	log.Printf("SELF_CHECK_OK addr=%s", listener.Addr().String())
	return nil
}

func waitForShutdown(server *http.Server, serveErrors <-chan error, timeout time.Duration) error {
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErrors:
		return err
	case <-signalContext.Done():
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("优雅关闭失败：%w", err)
	}
	return <-serveErrors
}
