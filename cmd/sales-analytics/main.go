package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	budget "github.com/drivelogic1-svg/Budget"
	"github.com/drivelogic1-svg/Budget/internal/server"
)

func main() {
	serverMode := flag.Bool("server", false, "Запустить в режиме веб-сервера без открытия настольного окна")
	hostFlag := flag.String("host", "", "Хост прослушивания (по умолчанию 127.0.0.1 для GUI и 0.0.0.0 для -server)")
	portFlag := flag.Int("port", 0, "TCP-порт (0 = автоматический свободный порт)")
	fileFlag := flag.String("file", "", "Путь к файлу Excel (.xlsx) для загрузки при старте")
	flag.Parse()

	initialFile := *fileFlag
	if initialFile == "" && flag.NArg() > 0 {
		initialFile = flag.Arg(0)
	}
	if initialFile == "" {
		initialFile = discoverLocalXLSX()
	}

	defName, defBytes, _ := budget.ReadDefaultXLSX()

	host := *hostFlag
	if host == "" {
		if *serverMode {
			host = "0.0.0.0"
		} else {
			host = "127.0.0.1"
		}
	}
	port := *portFlag
	if port == 0 && *serverMode {
		port = 8080
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		showFatalError("Budget Sales Analytics — Ошибка запуска", fmt.Sprintf("Не удалось открыть локальный порт %s: %v", addr, err))
		os.Exit(1)
	}

	var stopOnce sync.Once
	stopCh := make(chan struct{})
	triggerShutdown := func() {
		stopOnce.Do(func() {
			close(stopCh)
		})
	}

	var picker server.NativeFilePickerFunc
	if hasNativePicker() {
		picker = pickExcelFileNative
	}

	appSrv, err := server.New(server.Options{
		WebFS:            budget.WebFS,
		DefaultFileName:  defName,
		DefaultFileBytes: defBytes,
		InitialFilePath:  initialFile,
		DesktopMode:      !*serverMode,
		NativePicker:     picker,
		OnShutdown:       triggerShutdown,
	})
	if err != nil {
		showFatalError("Budget Sales Analytics — Ошибка чтения данных", err.Error())
		os.Exit(1)
	}

	httpSrv := &http.Server{
		Handler:           appSrv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port
	localURL := fmt.Sprintf("http://127.0.0.1:%d", actualPort)

	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
			triggerShutdown()
		}
	}()

	if *serverMode {
		log.Printf("Budget Sales Analytics веб-интерфейс запущен на http://%s:%d", host, actualPort)
	} else {
		log.Printf("Открытие окна аналитической программы: %s", localURL)
		_ = launchDesktopGUI(localURL, func() {
			// Give a brief grace window in case browser handed off to an existing instance
			time.Sleep(3 * time.Second)
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
	case <-stopCh:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func discoverLocalXLSX() string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var firstMatch string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, "~$") || !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
				continue
			}
			full := filepath.Join(dir, name)
			if strings.Contains(name, "Первое полугодие") {
				return full
			}
			if firstMatch == "" {
				firstMatch = full
			}
		}
		if firstMatch != "" {
			return firstMatch
		}
	}
	return ""
}
