package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/node"
)

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
		path := filepath.Join(c.DataDir, "config.json")
		if err := node.SaveDefaultConfig(path, c); err != nil {
			fatal(err)
		}
		fmt.Println(path)
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
		if err := n.Start(ctx); err != nil {
			fatal(err)
		}
		api := n.ServeAPI()
		go func() {
			if err := api.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				n.Status()
			}
		}()
		select {
		case <-ctx.Done():
		case <-n.Done():
		}
		_ = api.Shutdown(context.Background())
		n.Stop()
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
		resp, err := http.Post("http://"+cfg.APIListenAddress+"/shutdown", "application/json", nil)
		if err != nil {
			fatal(err)
		}
		_ = resp.Body.Close()
		fmt.Println("shutdown requested")
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
		resp, err := http.Get("http://" + cfg.APIListenAddress + "/status")
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
		resp, err := http.Get("http://" + cfg.APIListenAddress + "/peers")
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
	fmt.Println("syjd init [data-dir]\nsyjd start [data-dir]\nsyjd stop [data-dir]\nsyjd status [data-dir]\nsyjd peers [data-dir]")
}
func fatal(e error) { fmt.Fprintln(os.Stderr, "error:", e); os.Exit(1) }
