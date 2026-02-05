package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set via ldflags during build
	version = "dev"

	// Server flags
	serverPort string

	// Agent flags
	agentID      string
	serverURL    string
	stunServer   string
	agentTimeout int
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "remotty",
		Short:   "Remotty - Terminal management over WebRTC",
		Long:    "Remotty allows you to manage terminal sessions over WebRTC with a centralized server.",
		Version: version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	// Server command
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Server management commands",
	}

	serverStartCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the manager server",
		Long:  "Start the Remotty manager server to accept agent registrations and handle connections.",
		Example: `  remotty server start
  remotty server start --port 3000`,
		Run: func(cmd *cobra.Command, args []string) {
			if serverPort == "" {
				serverPort = os.Getenv("PORT")
			}
			if err := startServer(serverPort); err != nil {
				fmt.Printf("Server failed: %s\n", err)
				os.Exit(1)
			}
		},
	}

	serverStartCmd.Flags().StringVar(&serverPort, "port", "", "Port to listen on (default: 8080, or PORT env var)")

	serverCmd.AddCommand(serverStartCmd)

	// Agent command
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent management commands",
	}

	agentRegisterCmd := &cobra.Command{
		Use:   "register",
		Short: "Register as an agent with the server",
		Long:  "Register this machine as an agent with a Remotty manager server.",
		Example: `  remotty agent register --id my-laptop --server http://localhost:8080
  remotty agent register --id my-laptop --server https://example.com --stun stun:custom.stun.com:19302`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if agentID == "" {
				return fmt.Errorf("--id is required")
			}
			if serverURL == "" {
				return fmt.Errorf("--server is required")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			rs := registerSession{
				id:      agentID,
				host:    serverURL,
				timeout: agentTimeout,
			}
			rs.stunServers = []string{stunServer}

			if err := rs.run(); err != nil {
				fmt.Printf("Registration failed: %s\n", err)
				os.Exit(1)
			}
		},
	}

	agentRegisterCmd.Flags().StringVar(&agentID, "id", "", "Host ID for registration (required)")
	agentRegisterCmd.Flags().StringVar(&serverURL, "server", "", "Manager server URL (required, e.g., http://localhost:8080)")
	agentRegisterCmd.Flags().StringVar(&stunServer, "stun", "stun:stun.l.google.com:19302", "STUN server")
	agentRegisterCmd.Flags().IntVar(&agentTimeout, "timeout", 0, "Connection timeout in seconds (0 = wait indefinitely)")

	agentCmd.AddCommand(agentRegisterCmd)

	// Add commands to root
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(agentCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
