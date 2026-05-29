package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tracewayapp/traceway/cli/internal/config"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/internal/state"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

const defaultURL = "https://cloud.tracewayapp.com"

// login-specific flag values
var (
	loginURL          string
	loginUsername     string
	loginPasswordFile bool
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against a Traceway instance and store the JWT",
		RunE:  runLogin,
	}
	cmd.Flags().StringVar(&loginURL, "url", "", "Traceway base URL (default: existing or "+defaultURL+")")
	cmd.Flags().StringVar(&loginUsername, "username", "", "Email address (default: existing or interactive prompt)")
	cmd.Flags().BoolVar(&loginPasswordFile, "password-stdin", false, "Read password from stdin instead of prompting")
	return cmd
}

func runLogin(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := state.Load()
	if err != nil {
		return err
	}

	profileName := resolveProfileName(st)

	existingCfg, hasCfg := cfg.Profiles[profileName]
	existingState, hasState := st.Profiles[profileName]

	url := loginURL
	if url == "" {
		if hasCfg {
			url = existingCfg.URL
		} else {
			url = defaultURL
		}
	}

	username := loginUsername
	if username == "" {
		if hasCfg {
			username = existingCfg.Username
		}
		if username == "" {
			username, err = promptUsername(cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
		}
	}

	password, err := readPassword(cmd.InOrStdin(), cmd.OutOrStdout(), loginPasswordFile)
	if err != nil {
		return err
	}

	mode := output.ResolveMode(flagOutput, output.StdoutIsTerminal())
	c := client.New(url)
	jwt, err := c.Login(ctx, username, password)
	if err != nil {
		return renderAPIError(cmd.ErrOrStderr(), mode, err, true)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]config.Profile{}
	}
	cfg.Profiles[profileName] = config.Profile{
		URL:      url,
		Username: username,
	}

	if st.Profiles == nil {
		st.Profiles = map[string]state.ProfileState{}
	}
	currentProject := ""
	if hasState {
		currentProject = existingState.CurrentProjectID
	}
	st.Profiles[profileName] = state.ProfileState{
		JWT:              jwt,
		CurrentProjectID: currentProject,
	}
	// First profile ever → set CurrentProfile pointer. Don't override on subsequent logins.
	if st.CurrentProfile == "" {
		st.CurrentProfile = profileName
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	if err := st.Save(); err != nil {
		return err
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s on %s (profile: %s)\n", username, url, profileName)
	return err
}

func promptUsername(in io.Reader, out io.Writer) (string, error) {
	_, _ = fmt.Fprint(out, "Username: ")
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readPassword(in io.Reader, out io.Writer, fromStdin bool) (string, error) {
	if fromStdin {
		r := bufio.NewReader(in)
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	// Interactive: read with no echo if stdin is a real terminal.
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		_, _ = fmt.Fprint(out, "Password: ")
		bytes, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(out)
		return string(bytes), err
	}
	// Fallback: line-based read (covers test injection).
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
