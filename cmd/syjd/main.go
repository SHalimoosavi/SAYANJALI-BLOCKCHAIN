package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/node"
)

// generateAPIAuthToken returns a random 32-byte, hex-encoded bearer token
// used to authenticate mutating API calls (/mine, /shutdown, POST
// /transactions). Generated once at `init` time and stored in config.json.
func generateAPIAuthToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// authedRequest issues an HTTP request carrying the configured bearer token,
// if one is set. Mutating CLI subcommands (stop, mine) use this instead of
// the bare http.Post/http.Get calls the read-only subcommands still use.
var apiHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

func authedRequest(method, url, token string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil) // #nosec G704 -- URL is constructed from the locally loaded node API configuration.
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return apiHTTPClient.Do(req) // #nosec G704 -- destination is the locally configured SYJ node API endpoint.
}

func apiBaseURL(cfg node.Config) string {
	scheme := "http"
	if cfg.APIUseTLS {
		scheme = "https"
	}
	return scheme + "://" + cfg.APIListenAddress
}

type startNode interface {
	Start(context.Context) error
	Done() <-chan struct{}
	Err() error
	Stop()
	ServeAPI() *http.Server
	Status() map[string]any
}

func runStart(ctx context.Context, cfg node.Config, n startNode) error {
	return runStartWith(ctx, cfg, n, func(api *http.Server) {
		go func() {
			var err error
			if cfg.APIUseTLS {
				err = api.ListenAndServeTLS(cfg.APITLSCertFile, cfg.APITLSKeyFile)
			} else {
				err = api.ListenAndServe()
			}
			if err != nil && err != http.ErrServerClosed {
				n.Status()
			}
		}()
	})
}

func runStartWith(ctx context.Context, cfg node.Config, n startNode, startAPI func(*http.Server)) error {
	if err := n.Start(ctx); err != nil {
		return err
	}
	api := n.ServeAPI()
	if startAPI != nil {
		startAPI(api)
	}
	select {
	case <-ctx.Done():
	case <-n.Done():
	}
	_ = api.Shutdown(context.Background())
	n.Stop()
	if err := n.Err(); err != nil {
		return err
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".sayanjali")
	switch os.Args[1] {
	case "init":
		dir := defaultDir
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		c := node.DefaultConfig(dir)
		token, err := generateAPIAuthToken()
		if err != nil {
			fatal(err)
		}
		c.APIAuthToken = token
		path := filepath.Join(c.DataDir, "config.json")
		if err := node.SaveDefaultConfig(path, c); err != nil {
			fatal(err)
		}
		fmt.Println(path)
		fmt.Println("API auth token (required for /mine, /shutdown, and POST /transactions):")
		fmt.Println(token)
		fmt.Println("This token is stored in config.json. Keep it secret; anyone holding it can mine to this node's address and shut it down.")
	case "start":
		dir := defaultDir
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		c := node.DefaultConfig(dir)
		cfg, err := node.LoadConfig(filepath.Join(dir, "config.json"), c)
		if err != nil {
			fatal(err)
		}
		n := node.New(cfg)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := runStart(ctx, cfg, n); err != nil {
			fatal(err)
		}
	case "stop":
		dir := defaultDir
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		c := node.DefaultConfig(dir)
		cfg, err := node.LoadConfig(filepath.Join(dir, "config.json"), c)
		if err != nil {
			fatal(err)
		}
		resp, err := authedRequest(http.MethodPost, apiBaseURL(cfg)+"/shutdown", cfg.APIAuthToken)
		if err != nil {
			fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			fatal(fmt.Errorf("shutdown endpoint returned %s", resp.Status))
		}
		fmt.Println("shutdown requested")
	case "mine":
		dir := defaultDir
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		c := node.DefaultConfig(dir)
		cfg, err := node.LoadConfig(filepath.Join(dir, "config.json"), c)
		if err != nil {
			fatal(err)
		}
		resp, err := authedRequest(http.MethodPost, apiBaseURL(cfg)+"/mine", cfg.APIAuthToken)
		if err != nil {
			fatal(err)
		}
		defer resp.Body.Close()
		var v any
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			fatal(err)
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
		if resp.StatusCode >= 300 {
			os.Exit(1)
		}
	case "status":
		dir := defaultDir
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		c := node.DefaultConfig(dir)
		cfg, err := node.LoadConfig(filepath.Join(dir, "config.json"), c)
		if err != nil {
			fatal(err)
		}
		resp, err := apiHTTPClient.Get(apiBaseURL(cfg) + "/status") // #nosec G704 -- destination is the locally loaded node API configuration.
		if err != nil {
			fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			fatal(fmt.Errorf("status endpoint returned %s", resp.Status))
		}
		var v any
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			fatal(err)
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
	case "peers":
		dir := defaultDir
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		c := node.DefaultConfig(dir)
		cfg, err := node.LoadConfig(filepath.Join(dir, "config.json"), c)
		if err != nil {
			fatal(err)
		}
		resp, err := apiHTTPClient.Get(apiBaseURL(cfg) + "/peers") // #nosec G704 -- destination is the locally loaded SYJ node API endpoint.
		if err != nil {
			fatal(err)
		}
		defer resp.Body.Close()
		var v any
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			fatal(err)
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
	case "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("syjd init [data-dir]\nsyjd start [data-dir]\nsyjd stop [data-dir]\nsyjd status [data-dir]\nsyjd peers [data-dir]\nsyjd mine [data-dir]")
}

func fatal(e error) { fmt.Fprintln(os.Stderr, "error:", e); os.Exit(1) }
