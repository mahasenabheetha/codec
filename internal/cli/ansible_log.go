package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/ansiblelog"
)

var (
	logJSON bool
	logAll  bool
)

var ansibleLogCmd = &cobra.Command{
	Use:   "log <file|->",
	Short: "Read a whole Ansible log from a pipeline: runs, failures, recap",
	Long: `log reads a whole log as a pipeline prints it — CI timestamps, Packer or
Compose prefixes, colour codes, shell output and several playbook runs
mixed together — and prints each run's failures with their reason and
where the task is defined, the recap, and the other output that has
errors. --all lists every task with its status and duration.

Exit status: 0 fine, 1 error, 2 the log shows a failure (a failed or
unreachable host, a run cut short, or errors in the output around it).`,
	Example: "  codec ansible log job.log\n  kubectl logs job/deploy | codec ansible log -\n  codec ansible log job.log --all",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var data []byte
		var err error
		name := args[0]
		if name == "-" {
			data, err = io.ReadAll(os.Stdin)
			name = "<stdin>"
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			return err
		}
		a := ansiblelog.Analyze(string(data))
		if logJSON {
			if err := emitJSON(a); err != nil {
				return err
			}
		} else {
			printLog(cmd.OutOrStdout(), name, a, logAll)
		}
		if a.Summary.FirstFailure != nil || a.Summary.Unreachable > 0 || a.Summary.Verdict != "" {
			return exitCode(2)
		}
		return nil
	},
}

func init() {
	ansibleLogCmd.Flags().BoolVar(&logJSON, "json", false, "print the whole analysis as JSON")
	ansibleLogCmd.Flags().BoolVar(&logAll, "all", false, "list every task, not only failures")
	ansibleCmd.AddCommand(ansibleLogCmd)
}

func printLog(w io.Writer, name string, a *ansiblelog.Analysis, all bool) {
	s := a.Summary
	head := []string{plural(s.Runs, "run"), plural(len(s.Hosts), "host"), plural(s.Tasks, "task")}
	if s.DurationMs > 0 {
		head = append(head, duration(s.DurationMs))
	}
	fmt.Fprintf(w, "%s: %s\n", name, strings.Join(head, " · "))
	if s.Runs > 0 {
		fmt.Fprintf(w, "  failed=%d unreachable=%d changed=%d ok=%d skipped=%d rescued=%d ignored=%d\n",
			s.Failed, s.Unreachable, s.Changed, s.OK, s.Skipped, s.Rescued, s.Ignored)
	}
	if s.Verdict != "" {
		fmt.Fprintf(w, "  %s\n", s.Verdict)
	}

	n := 0
	for _, b := range a.Blocks {
		if b.Run == nil {
			continue
		}
		n++
		r := b.Run
		meta := []string{fmt.Sprintf("lines %d–%d", b.From, b.To)}
		for _, m := range []string{r.Playbook, b.Title} {
			if m != "" {
				meta = append(meta, m)
			}
		}
		meta = append(meta, string(r.Status))
		if !r.Complete {
			meta = append(meta, "no recap")
		}
		if r.DurationMs > 0 {
			meta = append(meta, duration(r.DurationMs))
		}
		fmt.Fprintf(w, "\nRun %d · %s\n", n, strings.Join(meta, " · "))
		for _, note := range r.Notes {
			if note.Level == "error" {
				fmt.Fprintf(w, "  line %d: %s\n", note.Line, note.Text)
			}
		}
		for _, p := range r.Plays {
			for _, t := range p.Tasks {
				if all {
					printTaskLine(w, p.Name, t)
				}
				for _, res := range t.Results {
					if (res.Status != ansiblelog.Failed && res.Status != ansiblelog.Unreachable) || res.Ignored || res.Rescued {
						continue
					}
					fmt.Fprintf(w, "  ✗ %s › %s  [%s] %s  (line %d)\n", orDash(p.Name), taskName(t), res.Host, res.Status, res.Line)
					if res.Item != "" {
						fmt.Fprintf(w, "      item: %s\n", res.Item)
					}
					if res.Msg != "" {
						fmt.Fprintf(w, "      %s\n", res.Msg)
					}
					if t.Path != "" {
						fmt.Fprintf(w, "      defined in %s:%d\n", t.Path, t.PathLine)
					}
				}
				if t.Status == ansiblelog.Unfinished {
					fmt.Fprintf(w, "  … %s › %s  never finished  (line %d)\n", orDash(p.Name), taskName(t), t.Line)
				}
			}
		}
		if len(r.Recap) > 0 {
			fmt.Fprintln(w, "  Recap:")
			width := 0
			for _, h := range r.Recap {
				width = max(width, len(h.Host))
			}
			for _, h := range r.Recap {
				fmt.Fprintf(w, "    %-*s  ok=%d changed=%d unreachable=%d failed=%d skipped=%d rescued=%d ignored=%d\n",
					width, h.Host, h.OK, h.Changed, h.Unreachable, h.Failed, h.Skipped, h.Rescued, h.Ignored)
			}
		}
	}

	var bad []string
	for _, b := range a.Blocks {
		if b.Run == nil && b.Errors > 0 {
			where := fmt.Sprintf("lines %d–%d", b.From, b.To)
			if b.Title != "" {
				where += " [" + b.Title + "]"
			}
			bad = append(bad, fmt.Sprintf("    %s: %s", where, plural(b.Errors, "error line")))
		}
	}
	if s.Other > 0 {
		fmt.Fprintf(w, "\nOther output: %s, %d with errors\n", plural(s.Other, "block"), len(bad))
		for _, l := range bad {
			fmt.Fprintln(w, l)
		}
	}
}

func printTaskLine(w io.Writer, play string, t ansiblelog.TaskView) {
	d := ""
	if t.DurationMs > 0 {
		d = "  " + duration(t.DurationMs)
	}
	kind := ""
	if t.Handler {
		kind = "handler "
	}
	fmt.Fprintf(w, "  %-10s %s › %s%s%s\n", t.Status, orDash(play), kind, taskName(t), d)
}

func taskName(t ansiblelog.TaskView) string {
	if t.Role != "" {
		return t.Role + " : " + t.Name
	}
	return t.Name
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// duration prints milliseconds the way people read run times: 850ms,
// 12.4s, 3m05s, 1h23m.
func duration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", ms)
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
