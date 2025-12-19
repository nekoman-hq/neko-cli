package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var port int
var host string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the application server",
	Long: `Start the application server on the specified host and port.
   
   The serve command will start a web server that can handle requests
   and provide API endpoints for your application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if port < 1 || port > 65535 {
			fmt.Fprintf(os.Stderr, "Error: port must be between 1 and 65535\n")
			os.Exit(1)
		}
		command := exec.Command("git", "status")
		output, _ := command.CombinedOutput()
		fmt.Println(string(output))
		if verbose {
			fmt.Printf("Starting server on %s:%d\n", host, port)
			fmt.Println("Verbose mode enabled")
			fmt.Println("Configuration validated successfully")
		} else {
			fmt.Printf("Server starting on %s:%d\n", host, port)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to run the server on")
	serveCmd.Flags().StringVarP(&host, "host", "H", "localhost", "Host to bind the server to")
}
