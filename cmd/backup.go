package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	config   string
	db       string
	host     string
	user     string
	password string
	name     string
	storage  string // path to store backup in locally
	compress bool
	dburl    string
)
var backup = &cobra.Command{
	Use:   "backup",
	Short: "backup database",
	Long: `Create a backup of the specified database and store it locally or on a remote server.

	The backup command supports multiple database engines and allows you to configure
	output location, compression, and authentication options. If the backup process
	fails, dbackup will return a non-zero exit code with a clear error message.`,
	Run: func(cmd *cobra.Command, args []string) {},

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(args, "---------->")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(backup)

	backup.PersistentFlags().StringVar(&config, "config", "", "provide path to config file")
	backup.PersistentFlags().StringVar(&db, "db", "", "database type to backup (sqlite, postgresql, mongoDB, mysql) etc...")
	backup.PersistentFlags().StringVar(&host, "host", "", "database host")
	backup.PersistentFlags().StringVar(&password, "pasword", "", "database password")
	backup.PersistentFlags().StringVar(&name, "name", "", "database to backup")
	backup.PersistentFlags().StringVar(&user, "user", "", "database user")
	backup.PersistentFlags().StringVar(&storage, "storage", "", "database storage if storage is omitted backup will be stored in current working directory support both remote storage and local storage")
	backup.PersistentFlags().BoolVar(&compress, "compress", false, "compress database file if provided if omitted file does not get compressed")
	backup.PersistentFlags().StringVar(&dburl, "dburl", "", "other field ommited provide ease of passing database credentials")
}
