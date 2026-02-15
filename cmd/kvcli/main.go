package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shinerio/gopher-kv/pkg/client"
)

func main() {
	host := flag.String("h", "127.0.0.1", "server host")
	port := flag.Int("p", 6380, "server port")
	flag.Parse()

	cli := client.New(*host, *port)
	args := flag.Args()
	if len(args) > 0 {
		runCommand(cli, strings.Join(args, " "))
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("GopherKV CLI. type 'help' for commands")
	for {
		fmt.Print("kv> ")
		if !scanner.Scan() {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return
		}
		runCommand(cli, line)
	}
}

func runCommand(cli *client.Client, line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch strings.ToLower(parts[0]) {
	case "set":
		if len(parts) < 3 {
			fmt.Println("usage: set <key> <value> [ttl <seconds>]")
			return
		}
		ttl := int64(0)
		if len(parts) == 5 && strings.ToLower(parts[3]) == "ttl" {
			t, err := strconv.ParseInt(parts[4], 10, 64)
			if err != nil || t < 0 {
				fmt.Println("invalid ttl")
				return
			}
			ttl = t
		}
		if err := cli.Set(ctx, parts[1], []byte(parts[2]), ttl); err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Println("ok")
	case "get":
		if len(parts) != 2 {
			fmt.Println("usage: get <key>")
			return
		}
		val, ttl, err := cli.Get(ctx, parts[1])
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Printf("value=%s ttl=%d\n", string(val), ttl)
	case "del":
		if len(parts) != 2 {
			fmt.Println("usage: del <key>")
			return
		}
		if err := cli.Delete(ctx, parts[1]); err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Println("ok")
	case "exists":
		if len(parts) != 2 {
			fmt.Println("usage: exists <key>")
			return
		}
		ok, err := cli.Exists(ctx, parts[1])
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Println(ok)
	case "ttl":
		if len(parts) != 2 {
			fmt.Println("usage: ttl <key>")
			return
		}
		ttl, err := cli.TTL(ctx, parts[1])
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Println(ttl)
	case "stats":
		st, err := cli.Stats(ctx)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Printf("%v\n", st)
	case "snapshot":
		ret, err := cli.Snapshot(ctx)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Printf("%v\n", ret)
	case "help":
		fmt.Println("set <key> <value> [ttl <seconds>]")
		fmt.Println("get <key>")
		fmt.Println("del <key>")
		fmt.Println("exists <key>")
		fmt.Println("ttl <key>")
		fmt.Println("stats")
		fmt.Println("snapshot")
		fmt.Println("help")
		fmt.Println("exit | quit")
	default:
		fmt.Println("unknown command")
	}
}
