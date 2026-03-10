package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hieptran/copilot-proxy/auth"
	"github.com/hieptran/copilot-proxy/config"
	"github.com/hieptran/copilot-proxy/proxy"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "login":
		cmdLogin()
	case "logout":
		cmdLogout()
	case "status":
		cmdStatus()
	case "serve":
		cmdServe()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Copilot Proxy - Save GitHub Copilot premium requests")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  copilot-proxy <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  login     Authenticate with GitHub Copilot (OAuth device flow)")
	fmt.Println("  logout    Clear stored authentication tokens")
	fmt.Println("  status    Show current authentication status")
	fmt.Println("  serve     Start the proxy server")
	fmt.Println("  help      Show this help message")
	fmt.Println()
	fmt.Println("Serve options:")
	fmt.Printf("  -p PORT   Port to listen on (default: %d)\n", config.DefaultPort)
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  copilot-proxy login")
	fmt.Println("  copilot-proxy serve")
	fmt.Println("  copilot-proxy serve -p 9090")
}

func cmdLogin() {
	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)
	if err := authenticator.Login(); err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdLogout() {
	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)
	if err := authenticator.Logout(); err != nil {
		fmt.Fprintf(os.Stderr, "Logout failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully logged out. All tokens have been cleared.")
}

func cmdStatus() {
	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)
	authenticated, expiry := authenticator.Status()

	if !authenticated {
		fmt.Println("Status: Not authenticated")
		fmt.Println("Run 'copilot-proxy login' to authenticate.")
		return
	}

	fmt.Println("Status: Authenticated")
	if !expiry.IsZero() {
		remaining := time.Until(expiry)
		if remaining > 0 {
			fmt.Printf("Copilot token expires in: %s\n", remaining.Round(time.Second))
		} else {
			fmt.Println("Copilot token: expired (will auto-refresh on next request)")
		}
		fmt.Printf("Token expiry: %s\n", expiry.Format("2006-01-02 15:04:05"))
	}
}

func cmdServe() {
	port := config.DefaultPort

	// Parse -p flag
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "-p" && i+1 < len(os.Args) {
			p, err := strconv.Atoi(os.Args[i+1])
			if err != nil || p < 1 || p > 65535 {
				fmt.Fprintf(os.Stderr, "Invalid port: %s\n", os.Args[i+1])
				os.Exit(1)
			}
			port = p
			i++ // skip next arg
		}
	}

	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !store.HasOAuthToken() {
		fmt.Fprintf(os.Stderr, "Not authenticated. Run 'copilot-proxy login' first.\n")
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)

	server, err := proxy.NewServer(authenticator, port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
