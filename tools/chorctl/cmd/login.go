package cmd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	loginUser  string
	loginPrint bool
)

// tokenFile returns the path where 'chorctl login' stores the session token.
func tokenFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tutti", "token"), nil
}

// resolveToken returns the token from --token, then TUTTI_TOKEN, then the saved token file.
func resolveToken() string {
	if token != "" {
		return token
	}
	if t := strings.TrimSpace(os.Getenv("TUTTI_TOKEN")); t != "" {
		return t
	}
	path, err := tokenFile()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveToken(t string) (string, error) {
	path, err := tokenFile()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(t+"\n"), 0o600); err != nil {
		return "", err
	}
	return path, os.Chmod(path, 0o600)
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Message   string `json:"message"`
}

func doLogin(baseURL, username, password string, insecure bool) (*loginResponse, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("address %q must be a full http(s) URL of the management REST api", baseURL)
	}
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	client := &http.Client{Timeout: 30 * time.Second}
	if insecure {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit --insecure opt-in
		client.Transport = t
	}
	resp, err := client.Post(u.JoinPath("auth", "login").String(), "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var res loginResponse
	_ = json.Unmarshal(data, &res)
	if resp.StatusCode != http.StatusOK {
		msg := res.Message
		if msg == "" {
			msg = strings.TrimSpace(string(data))
		}
		return nil, fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, msg)
	}
	return &res, nil
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Logs in to the management API and stores the session token",
	Long: `Logs in to a management API with authentication enabled (api.auth.enabled).
The password is prompted without echo, or read from stdin when it is not a terminal.
By default the token is saved to ~/.config/tutti/token (mode 0600) and used by later
chorctl commands. Use --print to only print the token (e.g. export TUTTI_TOKEN=$(chorctl login --user admin --print)).`,
	Run: func(cmd *cobra.Command, args []string) {
		if loginUser == "" {
			logrus.Fatal("--user is required")
		}
		var password string
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprint(os.Stderr, "Password: ")
			pw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				logrus.WithError(err).Fatal("unable to read password")
			}
			password = string(pw)
		} else {
			pw, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
			if err != nil {
				logrus.WithError(err).Fatal("unable to read password from stdin")
			}
			password = strings.TrimRight(string(pw), "\r\n")
		}
		res, err := doLogin(address, loginUser, password, insecureTLS)
		if err != nil {
			logrus.WithError(err).Fatal("unable to login")
		}
		if loginPrint {
			fmt.Println(res.Token)
			return
		}
		path, err := saveToken(res.Token)
		if err != nil {
			logrus.WithError(err).Fatal("unable to save token")
		}
		fmt.Fprintf(os.Stderr, "Logged in as %s (%s), token valid until %s, saved to %s\n", res.Username, res.Role, res.ExpiresAt, path)
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	// Local flag intentionally shadows the root persistent --user (storage user).
	loginCmd.Flags().StringVar(&loginUser, "user", "", "management API username")
	loginCmd.Flags().BoolVar(&loginPrint, "print", false, "print the token to stdout instead of saving it")
}
