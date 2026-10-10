package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"megabrain/internal/api"
	"megabrain/internal/config"
	"megabrain/internal/mapper"
	"megabrain/internal/store"
)

func main() {
	dataDir := flag.String("data-dir", ".", "diretório de dados (userDir do cliente)")
	portFlag := flag.Int("port", config.DefaultPort, "porta preferida (fallback nas seguintes)")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("criar data-dir: %v", err)
	}

	dbPath := filepath.Join(*dataDir, "megabrain.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("abrir banco: %v", err)
	}
	defer st.Close()

	// Encerra junto com o client (pai) e em SIGTERM/SIGINT.
	watchParent()
	handleSignals(st)

	srv := api.New(st, mapper.New(st))

	ln, port, err := listen(*portFlag)
	if err != nil {
		log.Fatalf("porta: %v", err)
	}

	portFile := filepath.Join(*dataDir, "port")
	if err := os.WriteFile(portFile, []byte(fmt.Sprintf("%d", port)), 0o644); err != nil {
		log.Printf("aviso: não gravou %s: %v", portFile, err)
	}

	log.Printf("MegaBrain em http://127.0.0.1:%d (db=%s)", port, dbPath)
	if err := http.Serve(ln, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

// watchParent encerra o processo quando o pai (client) morre.
// Cobre Linux/macOS via reparenting; no Windows o Job Object cuida disso.
func watchParent() {
	parent := os.Getppid()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if os.Getppid() != parent {
				os.Exit(0)
			}
		}
	}()
}

// handleSignals fecha o banco de forma limpa ao receber SIGTERM/SIGINT.
func handleSignals(st *store.Store) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		st.Close()
		os.Exit(0)
	}()
}

// listen tenta a porta preferida e, se ocupada, as seguintes.
func listen(preferred int) (net.Listener, int, error) {
	for p := preferred; p < preferred+config.PortRange; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, p, nil
		}
	}
	return nil, 0, fmt.Errorf("nenhuma porta livre em %d..%d", preferred, preferred+config.PortRange-1)
}
