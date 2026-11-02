package evaluation

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// textSection writes one titled table to tw.
type textSection func(tw *tabwriter.Writer)

// WriteText renders the report as aligned tables for a terminal.
//
//	err := report.WriteText(os.Stdout)
func (r Report) WriteText(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	sections := []textSection{r.writePipeline, r.writeTransactions, r.writeScenarios, r.writeSequences, r.writeRules, r.writeCauses}
	for i, section := range sections {
		if i > 0 {
			fmt.Fprintln(tw)
		}
		section(tw)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("write text report: %w", err)
	}
	return nil
}

func (r Report) writePipeline(tw *tabwriter.Writer) {
	p := r.Pipeline
	fmt.Fprintln(tw, "== Pipeline")
	fmt.Fprintf(tw, "truth lines\t%d\t(not accepted: %d)\n", p.TruthLines, p.NotAccepted)
	fmt.Fprintf(tw, "accepted\t%d\n", p.Accepted)
	fmt.Fprintf(tw, "analyzed\t%d\n", p.Analyzed)
	fmt.Fprintf(tw, "missing\t%d\t%s\n", p.Missing, formatCounts(p.MissingByScenario))
	fmt.Fprintf(tw, "redelivered\t%d\n", p.Redelivered)
	fmt.Fprintf(tw, "divergent\t%d\t%s\n", len(p.DivergentIDs), strings.Join(p.DivergentIDs, ", "))
}

func (r Report) writeTransactions(tw *tabwriter.Writer) {
	t := r.Transactions
	fmt.Fprintln(tw, "== Per transaction (flagged = review or block)")
	fmt.Fprintln(tw, "\tflagged\tapproved")
	fmt.Fprintf(tw, "fraud\t%d\t%d\n", t.TruePositive, t.FalseNegative)
	fmt.Fprintf(tw, "legit\t%d\t%d\n", t.FalsePositive, t.TrueNegative)
	fmt.Fprintf(tw, "precision %.3f  recall %.3f  false positive rate %.4f\n", t.Precision, t.Recall, t.FalsePositiveRate)
}

func (r Report) writeScenarios(tw *tabwriter.Writer) {
	fmt.Fprintln(tw, "== Decisions per scenario")
	fmt.Fprintln(tw, "SCENARIO\tLABEL\tAPPROVE\tREVIEW\tBLOCK")
	for _, s := range r.Scenarios {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\n", s.Scenario, s.Label, s.Approve, s.Review, s.Block)
	}
}

func (r Report) writeSequences(tw *tabwriter.Writer) {
	fmt.Fprintln(tw, "== Fraud sequences (scenario x card, detected if any transaction flagged)")
	fmt.Fprintln(tw, "SCENARIO\tDETECTED\tTOTAL")
	for _, s := range r.Sequences {
		fmt.Fprintf(tw, "%s\t%d\t%d\n", s.Scenario, s.Detected, s.Total)
	}
	fmt.Fprintf(tw, "sequence recall %.3f\n", r.SequenceRecall)
}

func (r Report) writeRules(tw *tabwriter.Writer) {
	fmt.Fprintln(tw, "== Rule firings")
	fmt.Fprintln(tw, "RULE\tFRAUD\tLEGIT")
	for _, rf := range r.Rules {
		fmt.Fprintf(tw, "%s\t%d\t%d\n", rf.Rule, rf.Fraud, rf.Legit)
	}
}

func (r Report) writeCauses(tw *tabwriter.Writer) {
	fmt.Fprintln(tw, "== Flagged legit transactions by cause")
	fmt.Fprintln(tw, "CAUSE\tRULES\tCOUNT")
	for _, c := range r.FalsePositiveCauses {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", c.Cause, c.Rules, c.Count)
	}
}

// formatCounts renders a map as "a=1 b=2" in key order.
func formatCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}
