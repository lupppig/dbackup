package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/web"
	"github.com/spf13/cobra"
)

var (
	uiPort int
)

func init() {
	uiCmd.Flags().IntVarP(&uiPort, "port", "p", 8080, "Port to run the UI server on")
	rootCmd.AddCommand(uiCmd)
}

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start the dbackup web UI and documentation server",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.New(logger.Config{
			JSON: rootCmd.Flag("log-json").Value.String() == "true",
		})

		uiSubFS, err := web.GetUIFS()
		if err != nil {
			l.Error("Failed to load embedded UI filesystem", "error", err)
			os.Exit(1)
		}

		docsSubFS, err := web.GetDocsFS()
		if err != nil {
			l.Error("Failed to load embedded Docs filesystem", "error", err)
			os.Exit(1)
		}

		mux := http.NewServeMux()

		// Serve the documentation at /docs/
		mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(docsSubFS))))

		// Serve the React UI at the root /
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if f, err := uiSubFS.Open(strings.TrimPrefix(path, "/")); err == nil {
				f.Close()
				http.FileServer(http.FS(uiSubFS)).ServeHTTP(w, r)
				return
			}

			// SPA fallback
			r.URL.Path = "/"
			http.FileServer(http.FS(uiSubFS)).ServeHTTP(w, r)
		})

		// Use the PORT environment variable if available (for Pxxl, Heroku, Render, etc.)
		port := uiPort
		if envPort := os.Getenv("PORT"); envPort != "" {
			fmt.Sscanf(envPort, "%d", &port)
		}

		addr := fmt.Sprintf(":%d", port)
		l.Info("Starting dbackup server", "url", fmt.Sprintf("http://localhost%s", addr), "docs", fmt.Sprintf("http://localhost%s/docs/", addr))

		if err := http.ListenAndServe(addr, mux); err != nil {
			l.Error("Server failed", "error", err)
			os.Exit(1)
		}
	},
}
