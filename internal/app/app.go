package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/OriginSparks/codex-usage-multi/internal/auth"
	"github.com/OriginSparks/codex-usage-multi/internal/profiles"
	"github.com/OriginSparks/codex-usage-multi/internal/usage"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		writeHelp(stdout)
		return nil
	}

	switch args[0] {
	case "add":
		return runAdd(args[1:], stdin, stdout, stderr)
	case "list":
		return runList(stdout)
	case "help", "--help", "-h":
		writeHelp(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText())
	}
}

func runAdd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: codex-usage add <email>")
	}
	email := strings.TrimSpace(strings.ToLower(args[0]))
	if err := validateEmail(email); err != nil {
		return err
	}

	created := false
	if !profileExists(email) {
		if err := profiles.Add(email); err != nil {
			return err
		}
		created = true
	}
	if created {
		_, _ = fmt.Fprintf(stdout, "Created profile %s\n", email)
	}

	info := profiles.Info(email)
	if _, err := os.Stat(info.AuthPath); err == nil {
		_, _ = fmt.Fprintf(stdout, "Profile %s is already logged in\n", email)
		return nil
	}

	if err := loginProfile(email, stdin, stdout, stderr); err != nil {
		return err
	}
	return nil
}

func runList(stdout io.Writer) error {
	names, err := profiles.List()
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if validateEmail(name) == nil {
			filtered = append(filtered, name)
		}
	}
	sort.Strings(filtered)

	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROFILE\t5H LEFT\t5H RESET\t1W LEFT\t1W RESET")
	for _, name := range filtered {
		row, err := buildListRow(name)
		if err != nil {
			fmt.Fprintf(tw, "%s\tERR\t-\tERR\t%s\n", name, compactError(err))
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", row.Profile, row.FiveHourLeft, row.FiveHourReset, row.WeekLeft, row.WeekReset)
	}
	return tw.Flush()
}

func buildListRow(name string) (listRow, error) {
	info := profiles.Info(name)
	token, _, err := auth.ExtractBearerTokenFromFile(info.AuthPath)
	if err != nil {
		return listRow{}, err
	}
	snapshot, err := usage.Fetch(name, token)
	if err != nil {
		return listRow{}, err
	}
	_ = profiles.MarkChecked(name)

	row := listRow{Profile: name, FiveHourLeft: "-", FiveHourReset: "-", WeekLeft: "-", WeekReset: "-"}
	for _, window := range snapshot.Windows {
		switch window.Label {
		case "5h":
			row.FiveHourLeft = fmt.Sprintf("%d%%", 100-window.UsedPercent)
			row.FiveHourReset = formatReset(window.ResetAt)
		case "1w":
			row.WeekLeft = fmt.Sprintf("%d%%", 100-window.UsedPercent)
			row.WeekReset = formatReset(window.ResetAt)
		}
	}
	return row, nil
}

func loginProfile(name string, stdin io.Reader, stdout, stderr io.Writer) error {
	info := profiles.Info(name)
	if err := os.MkdirAll(info.CodexHome, 0o755); err != nil {
		return err
	}
	if _, err := exec.LookPath("codex"); err != nil {
		return errors.New("could not find installed 'codex' CLI in PATH")
	}

	_, _ = fmt.Fprintf(stdout, "Starting login for %s\n", name)
	_, _ = fmt.Fprintf(stdout, "Using isolated CODEX_HOME=%s\n", info.CodexHome)

	cmd := exec.Command("codex", "login")
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "CODEX_HOME="+info.CodexHome)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex login failed: %w", err)
	}
	if _, err := os.Stat(info.AuthPath); err == nil {
		_, _ = fmt.Fprintf(stdout, "Login successful for %s\n", name)
		return nil
	}
	return fmt.Errorf("codex login finished, but no auth.json was found at %s", info.AuthPath)
}

func validateEmail(email string) error {
	if !emailPattern.MatchString(email) {
		return fmt.Errorf("invalid email: %s", email)
	}
	return nil
}

func profileExists(name string) bool {
	_, err := os.Stat(profiles.Info(name).RootDir)
	return err == nil
}

type listRow struct {
	Profile       string
	FiveHourLeft  string
	FiveHourReset string
	WeekLeft      string
	WeekReset     string
}

func formatReset(value string) string {
	if value == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	now := time.Now()
	local := t.In(now.Location())
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return local.Format("15:04")
	}
	if local.Year() == now.Year() {
		return local.Format("Jan 2")
	}
	return local.Format("2006-01-02")
}

func compactError(err error) string {
	msg := err.Error()
	if len(msg) > 40 {
		return msg[:40] + "..."
	}
	return msg
}

func writeHelp(stdout io.Writer) {
	fmt.Fprint(stdout, helpText())
}

func helpText() string {
	return strings.TrimSpace(`
codex-usage manages isolated Codex accounts by email.

Commands:
  add <email>
  list
`) + "\n"
}
