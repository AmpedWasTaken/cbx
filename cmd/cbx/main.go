package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/AmpedWasTaken/cbx/internal/model"
	"github.com/AmpedWasTaken/cbx/internal/payload"
	"github.com/AmpedWasTaken/cbx/internal/server"
	"github.com/AmpedWasTaken/cbx/internal/state"
	"github.com/AmpedWasTaken/cbx/internal/store"
	"github.com/AmpedWasTaken/cbx/internal/tunnel"
)

const version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printHelp()
		return nil
	}

	switch os.Args[1] {
	case "serve":
		return runServe(os.Args[2:])
	case "new":
		return runNew(os.Args[2:])
	case "list", "ls":
		return runList(os.Args[2:])
	case "inspect":
		return runInspect(os.Args[2:])
	case "watch":
		return runWatch(os.Args[2:])
	case "payload":
		return runPayload(os.Args[2:])
	case "report":
		return runReport(os.Args[2:])
	case "doctor":
		return runDoctor(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("cbx", version)
		return nil
	case "help", "--help", "-h":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

func cbxHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv("CBX_HOME")); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cbx"), nil
}

func openStore() (*store.Store, string, error) {
	home, err := cbxHome()
	if err != nil {
		return nil, "", err
	}
	st := store.New(home)
	if err := st.Init(); err != nil {
		return nil, "", err
	}
	return st, home, nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 7331, "local callback listener port")
	localOnly := fs.Bool("local", false, "disable the public tunnel")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, home, err := openStore()
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	httpServer := &http.Server{
		Handler:           server.New(st),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := httpServer.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	localURL := "http://" + addr
	publicURL := localURL
	tunnelName := "local"

	if !*localOnly {
		fmt.Println("Starting Cloudflare Quick Tunnel...")
		result, err := tunnel.StartCloudflareQuick(ctx, *port)
		if err != nil {
			_ = httpServer.Shutdown(context.Background())
			return fmt.Errorf("start public tunnel: %w", err)
		}
		publicURL = result.URL
		tunnelName = "cloudflare-quick"
	}

	if err := state.Save(home, state.State{
		BaseURL:   publicURL,
		LocalURL:  localURL,
		Port:      *port,
		Tunnel:    tunnelName,
		StartedAt: time.Now().UTC(),
	}); err != nil {
		_ = httpServer.Shutdown(context.Background())
		return err
	}
	defer state.Remove(home)

	fmt.Println()
	fmt.Println("CBX ONLINE")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("Local    %s\n", localURL)
	if !*localOnly {
		fmt.Printf("Public   %s\n", publicURL)
	}
	fmt.Printf("Data     %s\n", filepath.Join(home, "data"))
	fmt.Println()
	fmt.Println("Create a callback in another terminal:")
	fmt.Println("  cbx new --type xss")
	fmt.Println()
	fmt.Println("Waiting for callbacks... Ctrl+C to stop.")

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	return httpServer.Shutdown(shutdownCtx)
}

func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	name := fs.String("name", "", "human-friendly callback name")
	kind := fs.String("type", "http", "callback type")
	host := fs.String("host", "", "expected target host for scope correlation")
	ttl := fs.Duration("ttl", time.Hour, "callback lifetime")
	once := fs.Bool("once", false, "disable callback after its first event")
	jsonOut := fs.Bool("json", false, "print JSON")
	plain := fs.Bool("plain", false, "print only the callback URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ttl <= 0 {
		return errors.New("ttl must be greater than zero")
	}

	st, home, err := openStore()
	if err != nil {
		return err
	}

	id, err := randomToken(12)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	cb := model.Callback{
		ID:           id,
		Name:         strings.TrimSpace(*name),
		Type:         strings.ToLower(strings.TrimSpace(*kind)),
		ExpectedHost: strings.TrimSpace(*host),
		CreatedAt:    now,
		ExpiresAt:    now.Add(*ttl),
		Once:         *once,
	}
	if err := st.CreateCallback(cb); err != nil {
		return err
	}

	baseURL := "http://127.0.0.1:7331"
	if s, err := state.Load(home); err == nil && s.BaseURL != "" {
		baseURL = s.BaseURL
	}
	url := strings.TrimRight(baseURL, "/") + "/c/" + cb.ID

	if *plain {
		fmt.Println(url)
		return nil
	}
	if *jsonOut {
		return printJSON(map[string]any{
			"id": cb.ID, "name": cb.Name, "type": cb.Type, "url": url,
			"expiresAt": cb.ExpiresAt, "once": cb.Once, "expectedHost": cb.ExpectedHost,
		})
	}

	fmt.Println("Callback created")
	fmt.Println()
	fmt.Printf("ID       %s\n", cb.ID)
	if cb.Name != "" {
		fmt.Printf("Name     %s\n", cb.Name)
	}
	fmt.Printf("Type     %s\n", cb.Type)
	fmt.Printf("TTL      %s\n", time.Until(cb.ExpiresAt).Round(time.Second))
	if cb.ExpectedHost != "" {
		fmt.Printf("Scope    %s\n", cb.ExpectedHost)
	}
	fmt.Printf("URL      %s\n", url)
	return nil
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, _, err := openStore()
	if err != nil {
		return err
	}
	callbacks, err := st.ListCallbacks()
	if err != nil {
		return err
	}
	sort.Slice(callbacks, func(i, j int) bool { return callbacks[i].CreatedAt.After(callbacks[j].CreatedAt) })
	if *jsonOut {
		return printJSON(callbacks)
	}
	if len(callbacks) == 0 {
		fmt.Println("No callbacks yet. Create one with: cbx new")
		return nil
	}
	fmt.Printf("%-18s %-10s %-10s %-8s %s\n", "ID", "TYPE", "STATUS", "HITS", "NAME")
	for _, cb := range callbacks {
		events, _ := st.ListEvents(cb.ID)
		fmt.Printf("%-18s %-10s %-10s %-8d %s\n", cb.ID, cb.Type, callbackStatus(cb), len(events), cb.Name)
	}
	return nil
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: cbx inspect <id>")
	}
	st, _, err := openStore()
	if err != nil {
		return err
	}
	cb, err := st.GetCallback(fs.Arg(0))
	if err != nil {
		return err
	}
	events, err := st.ListEvents(cb.ID)
	if err != nil {
		return err
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ReceivedAt.Before(events[j].ReceivedAt) })

	if *jsonOut {
		return printJSON(map[string]any{"callback": cb, "events": events})
	}

	fmt.Printf("Callback %s\n", cb.ID)
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("Status       %s\n", callbackStatus(cb))
	fmt.Printf("Type         %s\n", cb.Type)
	if cb.Name != "" {
		fmt.Printf("Name         %s\n", cb.Name)
	}
	if cb.ExpectedHost != "" {
		fmt.Printf("Scope        %s\n", cb.ExpectedHost)
	}
	fmt.Printf("Created      %s\n", cb.CreatedAt.Local().Format(time.RFC3339))
	fmt.Printf("Expires      %s\n", cb.ExpiresAt.Local().Format(time.RFC3339))
	fmt.Printf("Hits         %d\n", len(events))
	if len(events) > 0 {
		first, last := events[0], events[len(events)-1]
		fmt.Printf("First hit    %s\n", first.ReceivedAt.Local().Format(time.RFC3339))
		fmt.Printf("Last hit     %s\n", last.ReceivedAt.Local().Format(time.RFC3339))
		fmt.Println()
		fmt.Println("Latest event")
		fmt.Printf("Method       %s\n", last.Method)
		fmt.Printf("Source IP    %s\n", last.SourceIP)
		if last.PageURL != "" {
			fmt.Printf("Page         %s\n", last.PageURL)
		}
		if last.Origin != "" {
			fmt.Printf("Origin       %s\n", last.Origin)
		}
		if last.UserAgent != "" {
			fmt.Printf("User-Agent   %s\n", last.UserAgent)
		}
		if last.ScopeMatch != nil {
			fmt.Printf("Scope match  %t\n", *last.ScopeMatch)
		}
		fmt.Printf("SHA256       %s\n", last.EvidenceHash)
	}
	return nil
}

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.Duration("interval", 750*time.Millisecond, "poll interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, _, err := openStore()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	seen := map[string]struct{}{}
	fmt.Println("CBX LIVE")
	fmt.Println("Waiting for events... Ctrl+C to stop.")
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			events, err := st.ListEvents("")
			if err != nil {
				return err
			}
			sort.Slice(events, func(i, j int) bool { return events[i].ReceivedAt.Before(events[j].ReceivedAt) })
			for _, event := range events {
				if _, ok := seen[event.ID]; ok {
					continue
				}
				seen[event.ID] = struct{}{}
				fmt.Printf("%s  %-18s %-6s %s\n",
					event.ReceivedAt.Local().Format("15:04:05"),
					event.CallbackID,
					event.Method,
					firstNonEmpty(event.PageURL, event.Origin, event.Path),
				)
			}
		}
	}
}

func runPayload(args []string) error {
	fs := flag.NewFlagSet("payload", flag.ContinueOnError)
	kind := fs.String("type", "xss-fetch", "xss-fetch, xss-event, xss-img, ssrf, webhook, curl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: cbx payload <id> [--type xss-fetch]")
	}
	st, home, err := openStore()
	if err != nil {
		return err
	}
	cb, err := st.GetCallback(fs.Arg(0))
	if err != nil {
		return err
	}
	s, err := state.Load(home)
	if err != nil {
		return errors.New("no active CBX server state; start cbx serve first")
	}
	out, err := payload.Generate(s.BaseURL, cb.ID, *kind)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: cbx report <id>")
	}
	st, _, err := openStore()
	if err != nil {
		return err
	}
	cb, err := st.GetCallback(fs.Arg(0))
	if err != nil {
		return err
	}
	events, err := st.ListEvents(cb.ID)
	if err != nil {
		return err
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ReceivedAt.Before(events[j].ReceivedAt) })

	fmt.Println("# CBX Callback Evidence")
	fmt.Println()
	fmt.Printf("- **Callback:** %s\n", cb.ID)
	fmt.Printf("- **Type:** %s\n", cb.Type)
	fmt.Printf("- **Status:** %s\n", callbackStatus(cb))
	fmt.Printf("- **Created:** %s\n", cb.CreatedAt.Format(time.RFC3339))
	fmt.Printf("- **Hits:** %d\n", len(events))
	if cb.ExpectedHost != "" {
		fmt.Printf("- **Expected host:** %s\n", cb.ExpectedHost)
	}
	if len(events) > 0 {
		fmt.Println()
		fmt.Println("## Events")
		fmt.Println()
		for _, event := range events {
			fmt.Printf("### %s\n\n", event.ReceivedAt.Format(time.RFC3339))
			fmt.Printf("- Method: %s\n", event.Method)
			if event.PageURL != "" {
				fmt.Printf("- Page: %s\n", event.PageURL)
			}
			if event.Origin != "" {
				fmt.Printf("- Origin: %s\n", event.Origin)
			}
			fmt.Printf("- Evidence SHA256: %s\n", event.EvidenceHash)
			fmt.Println()
		}
	}
	return nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	port := fs.Int("port", 7331, "port to test")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, home, err := openStore()
	if err != nil {
		return err
	}

	fmt.Println("CBX Doctor")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("Data directory   ✓ %s\n", home)

	if path, err := tunnel.CloudflaredPath(); err == nil {
		fmt.Printf("cloudflared      ✓ %s\n", path)
	} else {
		fmt.Println("cloudflared      ✗ not found in PATH")
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("Port %-5d       ✗ unavailable\n", *port)
	} else {
		fmt.Printf("Port %-5d       ✓ available\n", *port)
		_ = ln.Close()
	}

	if _, err := state.Load(home); err == nil {
		fmt.Println("Server state     ✓ active state file found")
	} else {
		fmt.Println("Server state     - no active state file")
	}
	return nil
}

func randomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func callbackStatus(cb model.Callback) string {
	now := time.Now()
	if cb.DisabledAt != nil {
		return "disabled"
	}
	if now.After(cb.ExpiresAt) {
		return "expired"
	}
	return "active"
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "-"
}

func printHelp() {
	fmt.Println("CBX - disposable security callbacks from your terminal")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cbx <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve     start the local collector and optional public tunnel")
	fmt.Println("  new       create a disposable callback")
	fmt.Println("  list      list callbacks")
	fmt.Println("  inspect   inspect one callback and its evidence")
	fmt.Println("  watch     stream newly received events")
	fmt.Println("  payload   generate a non-destructive callback proof")
	fmt.Println("  report    print a Markdown evidence report")
	fmt.Println("  doctor    check local prerequisites")
	fmt.Println("  version   print the version")
}
