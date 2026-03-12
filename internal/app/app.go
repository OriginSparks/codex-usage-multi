package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/OriginSparks/codex-usage-multi/internal/auth"
	"github.com/OriginSparks/codex-usage-multi/internal/profiles"
	"github.com/OriginSparks/codex-usage-multi/internal/usage"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const defaultRefreshInterval = time.Hour

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runDashboard(stdin, stdout)
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
	rows, err := fetchRows()
	if err != nil {
		return err
	}
	return printRows(stdout, rows)
}

func runDashboard(stdin io.Reader, stdout io.Writer) error {
	commands := make(chan string, 4)
	go readDashboardCommands(stdin, commands)

	rows, err := fetchRows()
	message := ""
	if err != nil {
		message = err.Error()
	}
	renderDashboard(stdout, rows, message)

	ticker := time.NewTicker(defaultRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case cmd, ok := <-commands:
			if !ok {
				return nil
			}
			switch cmd {
			case "q", "quit", "exit":
				return nil
			case "r", "refresh", "":
				rows, err = fetchRows()
				if err != nil {
					message = err.Error()
				} else {
					message = "refreshed"
				}
				renderDashboard(stdout, rows, message)
			case "a", "add":
				message = "use: codex-usage add <email>"
				renderDashboard(stdout, rows, message)
			default:
				message = fmt.Sprintf("unknown command: %s", cmd)
				renderDashboard(stdout, rows, message)
			}
		case <-ticker.C:
			rows, err = fetchRows()
			if err != nil {
				message = err.Error()
			} else {
				message = "auto-refreshed"
			}
			renderDashboard(stdout, rows, message)
		}
	}
}

func readDashboardCommands(stdin io.Reader, commands chan<- string) {
	defer close(commands)
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		commands <- strings.TrimSpace(strings.ToLower(scanner.Text()))
	}
}

func renderDashboard(stdout io.Writer, rows []listRow, message string) {
	_, _ = fmt.Fprint(stdout, "\033[2J\033[H")
	_, _ = fmt.Fprintf(stdout, "codex-usage dashboard%52s\n\n", "updated "+time.Now().Format("15:04"))
	_ = printRows(stdout, rows)
	_, _ = fmt.Fprintln(stdout)
	_, _ = fmt.Fprintln(stdout, "Commands: r refresh | a add hint | q quit")
	_, _ = fmt.Fprintf(stdout, "Refresh interval: %s\n", defaultRefreshInterval)
	if message != "" {
		_, _ = fmt.Fprintf(stdout, "Status: %s\n", message)
	}
	_, _ = fmt.Fprintln(stdout, "Type a command then press Enter.")
}

func printRows(stdout io.Writer, rows []listRow) error {
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROFILE\t5H LEFT\t5H RESET\t1W LEFT\t1W RESET\tSTATUS")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", row.Profile, row.FiveHourLeft, row.FiveHourReset, row.WeekLeft, row.WeekReset, row.Status)
	}
	return tw.Flush()
}

func fetchRows() ([]listRow, error) {
	names, err := profiles.List()
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if validateEmail(name) == nil {
			filtered = append(filtered, name)
		}
	}
	sort.Strings(filtered)
	if len(filtered) == 0 {
		return []listRow{}, nil
	}

	results := make([]listRow, len(filtered))
	var wg sync.WaitGroup
	for i, name := range filtered {
		wg.Add(1)
		go func(idx int, profile string) {
			defer wg.Done()
			row, err := buildListRow(profile)
			if err != nil {
				results[idx] = listRow{Profile: profile, FiveHourLeft: "ERR", FiveHourReset: "-", WeekLeft: "ERR", WeekReset: "-", Status: compactError(err)}
				return
			}
			results[idx] = row
		}(i, name)
	}
	wg.Wait()
	return results, nil
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

	row := listRow{Profile: name, FiveHourLeft: "-", FiveHourReset: "-", WeekLeft: "-", WeekReset: "-", Status: "ok"}
	for _, window := range snapshot.Windows {
		switch window.Label {
		case "5h":
			row.FiveHourLeft = fmt.Sprintf("%d%%", max(0, 100-window.UsedPercent))
			row.FiveHourReset = formatReset(window.ResetAt)
		case "1w":
			row.WeekLeft = fmt.Sprintf("%d%%", max(0, 100-window.UsedPercent))
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
	Status        string
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

No arguments starts the lightweight dashboard.
`) + "\n"
}
