package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shinerio/gopher-kv/pkg/client"
)

type CLI struct {
	client *client.Client
	prompt string
}

func NewCLI(host string, port int) *CLI {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	return &CLI{
		client: client.NewClient(baseURL),
		prompt: fmt.Sprintf("%s:%d> ", host, port),
	}
}

func (cli *CLI) printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  set <key> <value> [ttl <seconds>] - Set key-value pair")
	fmt.Println("  get <key>                         - Get value by key")
	fmt.Println("  del <key>                         - Delete key")
	fmt.Println("  exists <key>                      - Check if key exists")
	fmt.Println("  ttl <key>                         - Show key ttl")
	fmt.Println("  stats                             - Show server statistics")
	fmt.Println("  snapshot                          - Trigger RDB snapshot")
	fmt.Println("  help                              - Show this help")
	fmt.Println("  exit / quit                       - Exit the CLI")
}

func (cli *CLI) run() error {
	if err := cli.client.Health(); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	fmt.Println("Connected to GopherKV server")
	fmt.Println("Type 'help' for available commands")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(cli.prompt)
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return nil
		case "help":
			cli.printHelp()
		case "set":
			cli.handleSet(parts)
		case "get":
			cli.handleGet(parts)
		case "del", "delete":
			cli.handleDelete(parts)
		case "exists":
			cli.handleExists(parts)
		case "ttl":
			cli.handleTTL(parts)
		case "stats":
			cli.handleStats()
		case "snapshot":
			cli.handleSnapshot()
		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			fmt.Println("Type 'help' for available commands")
		}
	}

	return scanner.Err()
}

func (cli *CLI) handleSet(parts []string) {
	if len(parts) < 3 {
		fmt.Println("Usage: set <key> <value> [ttl <seconds>]")
		return
	}

	key := parts[1]
	value := parts[2]
	var ttl time.Duration

	if len(parts) >= 5 && parts[3] == "ttl" {
		sec, err := strconv.Atoi(parts[4])
		if err != nil {
			fmt.Println("Invalid TTL value")
			return
		}
		ttl = time.Duration(sec) * time.Second
	}

	if err := cli.client.Set(key, []byte(value), ttl); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("OK")
}

func (cli *CLI) handleGet(parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: get <key>")
		return
	}

	value, err := cli.client.Get(parts[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if value == nil {
		fmt.Println("(nil)")
		return
	}

	fmt.Printf("\"%s\"\n", string(value))
}

func (cli *CLI) handleDelete(parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: del <key>")
		return
	}

	if err := cli.client.Delete(parts[1]); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("OK")
}

func (cli *CLI) handleExists(parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: exists <key>")
		return
	}

	exists, err := cli.client.Exists(parts[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if exists {
		fmt.Println("(integer) 1")
	} else {
		fmt.Println("(integer) 0")
	}
}

func (cli *CLI) handleTTL(parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: ttl <key>")
		return
	}

	ttl, err := cli.client.TTL(parts[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("(integer) %d\n", ttl)
}

func (cli *CLI) handleStats() {
	stats, err := cli.client.Stats()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("keys=%d memory=%d hits=%d misses=%d uptime=%ds\n", stats.Keys, stats.Memory, stats.Hits, stats.Misses, stats.Uptime)
	fmt.Println("requests:")
	for op, cnt := range stats.Requests {
		fmt.Printf("  %s: %d\n", op, cnt)
	}
}

func (cli *CLI) handleSnapshot() {
	resp, err := cli.client.Snapshot()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("OK: %s (%s)\n", resp.Status, resp.Path)
}

func main() {
	host := flag.String("h", "localhost", "server host")
	port := flag.Int("p", 6380, "server port")
	flag.Parse()

	cli := NewCLI(*host, *port)
	if err := cli.run(); err != nil {
		log.Fatal(err)
	}
}
