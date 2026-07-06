package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hayou2002/clinepass-proxy/internal"
)

const version = "3.0.0"

func main() {
	host := flag.String("host", "127.0.0.1", "Bind address (0.0.0.0 for LAN access)")
	port := flag.Int("port", 55991, "Listen port")
	apiKey := flag.String("api-key", "", "ClinePass API Key (single-key mode, optional)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	thinkingLang := flag.String("thinking-lang", "zh", "Thinking language: zh or en")
	configPath := flag.String("config", "data/config.json", "Config file path (for pool mode)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("clinepass-proxy v%s\n", version)
		os.Exit(0)
	}

	// Create proxy instance (single key mode backward compat)
	proxy := internal.NewProxy(*apiKey, *debug, *thinkingLang)

	// Initialize key pool from config file
	pool := internal.NewKeyPool(*configPath)

	// Initialize model manager (needs pool for persistence)
	models := internal.NewModelManager(pool)

	// Connect pool and models to proxy
	proxy.Pool = pool
	proxy.Models = models

	// Server config for admin
	serverCfg := &internal.ServerConfig{
		ThinkingLang: *thinkingLang,
		Debug:        *debug,
		Host:         *host,
		Port:         *port,
	}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", proxy.HandleChatCompletions)
	mux.HandleFunc("/v1/models", proxy.HandleModels)
	mux.HandleFunc("/health", proxy.HandleHealth(version))

	// Admin web interface
	if pool != nil {
		admin := internal.NewAdminServer(pool, models, proxy, serverCfg)
		admin.RegisterRoutes(mux)
	}

	// Root redirect
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/admin/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	addr := fmt.Sprintf("%s:%d", *host, *port)

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Printf("║  ClinePass Proxy v%-7s                        ║\n", version)
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("║  Address:        %-35s║\n", addr)
	fmt.Printf("║  Thinking Lang:  %-35s║\n", *thinkingLang)
	fmt.Printf("║  Config:         %-35s║\n", *configPath)
	poolKeys := 0
	if pool != nil {
		poolKeys = len(pool.Keys)
	}
	fmt.Printf("║  Pool Keys:      %-35d║\n", poolKeys)
	if *apiKey != "" && len(*apiKey) > 8 {
		fmt.Printf("║  Fallback Key:   %-35s║\n", (*apiKey)[:8]+"...")
	}
	fmt.Printf("║  Debug:          %-35v║\n", *debug)
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Println("║  Endpoints:                                     ║")
	fmt.Printf("║  POST %-41s║\n", addr+"/v1/chat/completions")
	fmt.Printf("║  GET  %-41s║\n", addr+"/v1/models")
	fmt.Printf("║  GET  %-41s║\n", addr+"/admin/ (Web UI)")
	fmt.Printf("║  GET  %-41s║\n", addr+"/health")
	fmt.Println("╚══════════════════════════════════════════════════╝")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		if pool != nil {
			pool.Save()
		}
		os.Exit(0)
	}()

	log.Fatal(http.ListenAndServe(addr, mux))
}
