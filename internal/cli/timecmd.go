package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/cron"
	"github.com/mahasenabheetha/codec/v2/internal/timeutil"
)

// The Time tool's commands: time (timestamps) and cron. Logic lives in
// internal/timeutil and internal/cron.

var (
	timeZone string
	timeJSON bool
	cronRuns int
)

var timeCmd = &cobra.Command{
	Use:   "time [epoch|date]",
	Short: "Read an epoch timestamp or a date; with nothing, now",
	Long: `time shows a moment in UTC, in a zone, as epoch seconds and milliseconds,
and relative to now. An epoch number's unit is told by its size: up to 11
digits are seconds, then milliseconds, microseconds and nanoseconds. A
date without a zone is read in --zone.`,
	Example: "  codec time\n  codec time 1770120309123\n  codec time '2026-02-03 13:05' --zone Europe/Stockholm",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loc, err := timeutil.Zone(timeZone)
		if err != nil {
			return err
		}
		now := time.Now()
		var st *timeutil.Stamp
		if len(args) == 0 {
			st = timeutil.Describe(now, "now", loc, now)
		} else if st, err = timeutil.Parse(args[0], loc, now); err != nil {
			return err
		}
		if timeJSON {
			return emitJSON(st)
		}
		var b strings.Builder
		tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		for _, kv := range [][2]string{
			{"read as", st.Kind},
			{"unix", fmt.Sprint(st.Unix)},
			{"unix ms", fmt.Sprint(st.UnixMs)},
			{"utc", st.UTC},
			{st.Zone, st.Local},
			{"", st.Readable},
			{"relative", st.Relative},
			{"iso week", fmt.Sprintf("%s, day %d of the year", st.ISOWeek, st.DayOfYr)},
		} {
			fmt.Fprintf(tw, "%s\t%s\n", kv[0], kv[1])
		}
		tw.Flush()
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

var cronCmd = &cobra.Command{
	Use:   "cron <expression>",
	Short: "Explain a cron expression and list its next runs",
	Long: `cron explains a standard cron expression (5 fields, or @daily-style
shortcuts, optionally after CRON_TZ=Zone) in plain words, field by field,
and lists its next runs in --zone (UTC by default, as GitHub Actions and
Azure Pipelines run schedules). Quartz and Jenkins expressions are named
as such. Quote the expression so the shell leaves * alone.`,
	Example: "  codec cron '*/15 2-6 * * 1-5'\n  codec cron '30 2 * * *' --zone Europe/Stockholm -n 5\n  codec cron @weekly",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := cron.Parse(strings.Join(args, " "))
		if err != nil {
			return err
		}
		zone := timeZone
		if s.Zone != "" {
			zone = s.Zone
		}
		loc, err := timeutil.Zone(zone)
		if err != nil {
			return err
		}
		runs, skipped := s.Next(time.Now(), min(max(cronRuns, 1), 1000), loc)
		if timeJSON {
			return emitJSON(map[string]any{"description": s.Describe(), "schedule": s, "zone": loc.String(), "runs": runs, "skipped": skipped})
		}
		var b strings.Builder
		fmt.Fprintln(&b, s.Describe())
		if s.Standard != "" {
			fmt.Fprintf(&b, "(%s is %s)\n", s.Expr, s.Standard)
		}
		b.WriteByte('\n')
		tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		for _, f := range s.Fields {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Name, f.Text, f.Meaning())
		}
		tw.Flush()
		fmt.Fprintf(&b, "\nNext runs (%s):\n", loc)
		if len(runs) == 0 {
			b.WriteString("  never: no date matches (such as 30 February)\n")
		}
		for _, r := range runs {
			fmt.Fprintf(&b, "  %s  %s\n", r.Format("Mon 2006-01-02 15:04 MST"), timeutil.Relative(r, time.Now()))
		}
		for _, sk := range skipped {
			fmt.Fprintf(&b, "  skipped %s: the clocks jump over it (daylight saving)\n", sk.Wall)
		}
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

func init() {
	for _, c := range []*cobra.Command{timeCmd, cronCmd} {
		c.Flags().StringVar(&timeZone, "zone", "", `time zone: UTC (default), Local, or a name such as Europe/Stockholm`)
		c.Flags().BoolVar(&timeJSON, "json", false, "print as JSON")
	}
	cronCmd.Flags().IntVarP(&cronRuns, "count", "n", 10, "how many next runs to list")
	rootCmd.AddCommand(timeCmd, cronCmd)
}
