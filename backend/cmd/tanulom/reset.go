package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"tanulom/internal/auth"
	"tanulom/internal/db"
)

// runResetPassword implements `tanulom reset-password`: the way back in when the
// password is forgotten. It works on the data directory directly, so run it with
// the server stopped (or at least expect the running server to keep its old
// sessions until they are cleared below).
//
//	tanulom reset-password --data-dir ./data                 # the only account
//	tanulom reset-password --data-dir ./data --email a@b.c   # a specific one
//	tanulom reset-password --data-dir ./data --password '...'
//
// Without --password (or TANULOM_NEW_PASSWORD) a random one is generated and
// printed once. Every session of that account is deleted, so a stolen cookie
// stops working too. The learner's cards and history are untouched.
func runResetPassword(args []string) int {
	fs := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	var dataDir, email, password string
	fs.StringVar(&dataDir, "data-dir", "", "data directory (default: $TANULOM_DATA_DIR or ./data)")
	fs.StringVar(&email, "email", "", "account to reset (default: the only account, or the first admin)")
	fs.StringVar(&password, "password", "", "new password (min 10 characters); omitted = generate one")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if dataDir == "" {
		dataDir = os.Getenv("TANULOM_DATA_DIR")
	}
	if dataDir == "" {
		dataDir = "./data"
	}
	if password == "" {
		password = os.Getenv("TANULOM_NEW_PASSWORD")
	}

	path := db.Path(dataDir)
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "no database at %s — nothing to reset (a fresh start asks you to create the account in the browser)\n", path)
		return 1
	}
	d, err := db.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open %s: %v\n", filepath.Clean(path), err)
		return 1
	}
	defer d.Close()

	var id int64
	var found string
	if email != "" {
		norm, err := auth.NormalizeEmail(email)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad email: %v\n", err)
			return 2
		}
		err = d.QueryRow(`SELECT id, email FROM user WHERE email = ?`, norm).Scan(&id, &found)
		if err != nil {
			fmt.Fprintf(os.Stderr, "no account %q in %s\n", norm, path)
			listAccounts(d)
			return 1
		}
	} else {
		var n int
		_ = d.QueryRow(`SELECT COUNT(*) FROM user`).Scan(&n)
		if n == 0 {
			fmt.Fprintln(os.Stderr, "the database has no accounts yet — open the app and create one")
			return 1
		}
		if err := d.QueryRow(
			`SELECT id, email FROM user ORDER BY (role = 'admin') DESC, id LIMIT 1`,
		).Scan(&id, &found); err != nil {
			fmt.Fprintf(os.Stderr, "cannot read accounts: %v\n", err)
			return 1
		}
		if n > 1 {
			fmt.Fprintf(os.Stderr, "note: %d accounts exist; resetting the first admin. Use --email for another one:\n", n)
			listAccounts(d)
		}
	}

	generated := false
	if password == "" {
		p, err := randomPassword(16)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot generate a password: %v\n", err)
			return 1
		}
		password, generated = p, true
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 2
	}

	tx, err := d.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot start a transaction: %v\n", err)
		return 1
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE user SET password_hash = ? WHERE id = ?`, hash, id); err != nil {
		fmt.Fprintf(os.Stderr, "cannot update the password: %v\n", err)
		return 1
	}
	if _, err := tx.Exec(`DELETE FROM session WHERE user_id = ?`, id); err != nil {
		fmt.Fprintf(os.Stderr, "cannot clear sessions: %v\n", err)
		return 1
	}
	if err := tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot save: %v\n", err)
		return 1
	}

	fmt.Printf("password reset for %s (all sessions signed out)\n", found)
	if generated {
		fmt.Printf("new password: %s\n", password)
		fmt.Println("write it down, sign in, then change it in Settings.")
	}
	return 0
}

func listAccounts(d *db.DB) {
	rows, err := d.Query(`SELECT email, role FROM user ORDER BY id`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var e, r string
		if rows.Scan(&e, &r) == nil {
			fmt.Fprintf(os.Stderr, "  %s (%s)\n", e, r)
		}
	}
}

// randomPassword draws from an alphabet with no look-alike characters, so it can
// be read off a terminal and typed without guessing 0/O or l/1.
func randomPassword(n int) (string, error) {
	const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	out := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range out {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = alphabet[v.Int64()]
	}
	return string(out), nil
}
