// ABOUTME: Tail command that prints the newest entries, then each new entry as it is added.
// ABOUTME: It opens storage read-only and finds new entries through the storage feed, so it never writes.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/storage"
	"github.com/spf13/cobra"
)

// tailPollInterval is how often tail checks the store for new entries.
const tailPollInterval = time.Second

var (
	tailLimit int
	tailTags  []string
)

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Follow new entries as they are added",
	Long: `Print the newest entries, oldest first, then print each new entry as it is added, until Ctrl+C.

Tail opens storage read-only, so it is safe to leave running beside agents that write to the same log.`,
	Example: `  chronicle tail                  # The last 10 entries, then new ones
  chronicle tail -n 0             # New entries only
  chronicle tail -t work -t kata  # Entries tagged work or kata`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		reader, err := cfg.OpenReader()
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer func() { _ = reader.Close() }()

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		// After the first signal, restore the default signal behavior, so that a second Ctrl+C ends
		// a tail that is stuck writing to a pipe that nobody reads.
		context.AfterFunc(ctx, stop)
		return runTail(ctx, cmd.OutOrStdout(), reader, tailLimit, tailTags, tailPollInterval)
	},
}

// runTail prints the newest limit entries that carry any of the tags, oldest first, then polls the
// store every interval and prints each new entry until ctx ends. It returns nil when ctx ends. A
// limit of zero or less prints no history, and no tags means every entry. The interval must be
// positive.
func runTail(ctx context.Context, out io.Writer, reader storage.Reader, limit int, tags []string, interval time.Duration) error {
	filter := &storage.SearchFilter{Tags: tags}
	// The feed opens before the history is read, so an entry added in between is not missed. Such
	// an entry then shows up in both, which is why runTail remembers what it has printed.
	feed, err := reader.OpenFeed(filter)
	if err != nil {
		return fmt.Errorf("failed to open entry feed: %w", err)
	}

	home, _ := os.UserHomeDir() // Without a home folder, paths are shown in full.
	printed := make(map[string]bool)
	show := func(entry storage.Entry) error {
		if printed[entry.ID] {
			return nil
		}
		printed[entry.ID] = true
		if _, err := io.WriteString(out, formatTailEntry(entry, time.Now(), home)); err != nil {
			return fmt.Errorf("failed to write entry: %w", err)
		}
		return nil
	}

	if limit > 0 {
		history, err := reader.SearchEntries(filter, limit)
		if err != nil {
			return fmt.Errorf("failed to read recent entries: %w", err)
		}
		// SearchEntries returns the newest entry first.
		for i := len(history) - 1; i >= 0; i-- {
			if err := show(history[i]); err != nil {
				return err
			}
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fresh, err := feed.Poll()
			if err != nil {
				return fmt.Errorf("failed to poll for new entries: %w", err)
			}
			for _, entry := range fresh {
				if err := show(entry); err != nil {
					return err
				}
			}
		}
	}
}

// The colors of an entry's parts. They turn themselves off when stdout is not a terminal or
// NO_COLOR is set.
var (
	tailFaintColor = color.New(color.Faint) // The time and the working directory.
	tailTagColor   = color.New(color.FgCyan)
)

// formatTailEntry returns the text that tail prints for an entry. The first line holds the time,
// the first line of the message and the tags. Later lines of the message follow, then the working
// directory, each indented to line up under the message. The time is in now's time zone, and now
// decides how much of it shows. home is the home folder, which the working directory shortens to "~".
func formatTailEntry(entry storage.Entry, now time.Time, home string) string {
	when := tailTimeLabel(entry.Timestamp.In(now.Location()), now)
	indent := strings.Repeat(" ", len(when)+2)
	lines := strings.Split(entry.Message, "\n")

	var text strings.Builder
	fmt.Fprintf(&text, "%s  %s", tailFaintColor.Sprint(when), lines[0])
	if len(entry.Tags) > 0 {
		fmt.Fprintf(&text, "  %s", tailTagColor.Sprint("#"+strings.Join(entry.Tags, " #")))
	}
	text.WriteString("\n")
	for _, line := range lines[1:] {
		text.WriteString(indent + line + "\n")
	}
	if entry.WorkingDirectory != "" {
		text.WriteString(indent + tailFaintColor.Sprint(abbreviateHome(entry.WorkingDirectory, home)) + "\n")
	}
	return text.String()
}

// tailTimeLabel formats the time at, which is in now's time zone. An entry from today shows the time
// of day, one from another day this year adds the month and day, and an older one shows the date in full.
func tailTimeLabel(at, now time.Time) string {
	switch {
	case at.Year() != now.Year():
		return at.Format("2006-01-02 15:04:05")
	case at.YearDay() != now.YearDay():
		return at.Format("Jan 2 15:04:05")
	default:
		return at.Format("15:04:05")
	}
}

// abbreviateHome writes a path inside the home folder with "~" in place of the folder. A path
// outside it, or an unknown (empty) home folder, leaves the path as it is.
func abbreviateHome(path, home string) string {
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, found := strings.CutPrefix(path, home+"/"); found {
		return "~/" + rest
	}
	return path
}

func init() {
	tailCmd.Flags().IntVarP(&tailLimit, "limit", "n", 10, "Number of recent entries to show before following (0 for none)")
	tailCmd.Flags().StringArrayVarP(&tailTags, "tag", "t", []string{}, "Only show entries with this tag (repeat to allow several)")
	rootCmd.AddCommand(tailCmd)
}
