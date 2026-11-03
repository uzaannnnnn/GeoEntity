package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"geoentity/backend/internal/auth"
	"geoentity/backend/internal/backup"
	"geoentity/backend/internal/maintenance"
	"geoentity/backend/internal/repository"

	"golang.org/x/term"
	_ "modernc.org/sqlite"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) < 2 || args[0] != "admin" {
		return errors.New("usage: geoentity admin provision|reset-password|backup create|list|verify|restore --from <manifest>")
	}
	dbPath := env("GEOENTITY_DB_PATH", "./data/geoentity.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return err
	}
	switch args[1] {
	case "provision", "reset-password":
		if len(args) != 2 {
			return errors.New("password commands do not accept credentials as arguments")
		}
		lock, err := maintenance.Acquire(dbPath+".lock", true, true)
		if err != nil {
			return err
		}
		defer maintenance.Release(lock)
		password, err := promptPassword()
		if err != nil {
			return err
		}
		ctx := context.Background()
		store, err := repository.Open(ctx, dbPath)
		if err != nil {
			return err
		}
		defer store.Close()
		secure := os.Getenv("GEOENTITY_DEV_COOKIE_INSECURE") != "true"
		origin := env("GEOENTITY_PUBLIC_ORIGIN", "https://localhost:5173")
		svc, err := auth.New(store.DB(), auth.Config{PublicOrigin: origin, CookieSecure: secure})
		if err != nil {
			return err
		}
		if args[1] == "provision" {
			err = svc.Provision(ctx, password)
		} else {
			err = svc.RotatePassword(ctx, password)
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "Administrator credentials updated.")
		return nil
	case "backup":
		if len(args) < 3 {
			return errors.New("usage: geoentity admin backup create|list|verify [manifest]")
		}
		backupDir := env("GEOENTITY_BACKUP_DIR", "./backups")
		switch args[2] {
		case "list":
			if len(args) != 3 {
				return errors.New("usage: geoentity admin backup list")
			}
			items, err := backup.List(backupDir)
			if err != nil {
				return err
			}
			for _, m := range items {
				fmt.Printf("%s %s %d bytes sha256=%s\n", m.CreatedAt, m.DatabaseFile, m.SizeBytes, m.SHA256)
			}
			return nil
		case "verify":
			if len(args) != 4 {
				return errors.New("usage: geoentity admin backup verify <manifest.json>")
			}
			m, err := backup.Verify(context.Background(), args[3])
			if err != nil {
				return err
			}
			fmt.Printf("verified %s (%d bytes)\n", m.DatabaseFile, m.SizeBytes)
			return nil
		case "create":
			if len(args) != 3 {
				return errors.New("usage: geoentity admin backup create")
			}
			lock, err := maintenance.Acquire(dbPath+".lock", false, true)
			if err != nil {
				return err
			}
			defer maintenance.Release(lock)
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			keep, _ := strconv.Atoi(env("GEOENTITY_BACKUP_KEEP", "10"))
			m, err := backup.Create(context.Background(), db, backupDir, keep)
			if err != nil {
				return err
			}
			fmt.Printf("created verified backup %s\n", m.DatabaseFile)
			return nil
		default:
			return errors.New("unknown backup command")
		}
	case "restore":
		if len(args) != 4 || args[2] != "--from" {
			return errors.New("usage: geoentity admin restore --from <manifest.json>")
		}
		lock, err := maintenance.Acquire(dbPath+".lock", true, true)
		if err != nil {
			return err
		}
		defer maintenance.Release(lock)
		return backup.Restore(context.Background(), dbPath, args[3])
	default:
		return errors.New("unknown admin command")
	}
}
func promptPassword() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("password provisioning requires an interactive terminal")
	}
	fmt.Fprint(os.Stderr, "Passphrase: ")
	one, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		for i := range one {
			one[i] = 0
		}
		return "", err
	}
	fmt.Fprint(os.Stderr, "Repeat passphrase: ")
	two, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		for i := range one {
			one[i] = 0
		}
		return "", err
	}
	if string(one) != string(two) {
		for i := range one {
			one[i] = 0
		}
		for i := range two {
			two[i] = 0
		}
		return "", errors.New("passphrases do not match")
	}
	for i := range two {
		two[i] = 0
	}
	password := string(one)
	for i := range one {
		one[i] = 0
	}
	return password, nil
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
