package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hayou2002/clinepass-proxy/internal"
)

const version = "2.0.0"

func main() {
	host := flag.String("host", "127.0.0.1", "Bind address (0.0.0.0 for LAN access)")
	port := flag.Int("port", 55991, "Listen port")
	apiKey := flag.String("api-key", "", "ClinePass API Key (server-side, overrides client header)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("clinepass-proxy v%s\n", version)
		os.Exit(0)
	}

	proxy := internal.NewProxy(*apiKey, *debug)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", proxy.HandleChatCompletions)
	mux.HandleFunc("/v1/models", proxy.HandleModels)
	mux.HandleFunc("/health", proxy.HandleHealth(version))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<html><body style="font-family:monospace;padding:40px">
<h2>ClinePass Proxy v%s</h2>
<p>Endpoints:</p>
<ul>
<li>POST /v1/chat/completions</li>
<li>GET /v1/models</li>
<li>GET /health</li>
</ul>
<p>Configure your client: <code>http://%s:%d/v1</code></p>
</body></html>`, version, *host, *port)
			return
		}
		http.NotFound(w, r)
	})

	addr := fmt.Sprintf("%s:%d", *host, *port)

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Printf("║  ClinePass Proxy v%-7s               ║\n", version)
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Printf("║  Address:        %-24s║\n", addr)
	fmt.Printf("║  Debug:          %-24v║\n", *debug)
	keyDisplay := "(use client header)"
	if *apiKey != "" && len(*apiKey) > 8 {
		keyDisplay = (*apiKey)[:8] + "..."
	} else if *apiKey != "" {
		keyDisplay = "(set)"
	}
	fmt.Printf("║  API Key:        %-24s║\n", keyDisplay)
	fmt.Printf("║  Reasoning:      %-24s║\n", "medium (client override)")
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Println("║  10 ClinePass models                     ║")
	fmt.Println("║  reasoning | vision | tools | video      ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	log.Fatal(http.ListenAndServe(addr, mux))
}
