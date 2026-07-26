package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"mywholelife/dream"
	"mywholelife/store"
)

var (
	dreamAgentID string
	dreamScan    bool
	dreamRepair  bool
)

var dreamCmd = &cobra.Command{
	Use:   "dream",
	Short: "Run the long-term memory maintenance job",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dreamAgentID == "" {
			return fmt.Errorf("dream: --agent is required")
		}
		cfg := dream.FromEnv()
		blobs := store.NewBlobStore(cfg.DataRoot)
		st, err := store.Open(cfg.DBPath, blobs)
		if err != nil {
			return err
		}
		job := &dream.Job{
			Store: st,
			Blobs: blobs,
			Gen:   dream.NewLLMHookGen(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.Model),
			Cfg:   cfg,
			Now:   time.Now().Unix(),
		}
		ctx := cmd.Context()
		if dreamScan {
			report, err := job.Scan(ctx, dreamAgentID, dreamRepair)
			if err != nil {
				return err
			}
			fmt.Printf("dream scan: %d finding(s)\n", len(report.Findings))
			for _, f := range report.Findings {
				fmt.Printf("  %s memId=%s relPath=%s %s\n", f.Kind, f.MemID, f.RelPath, f.Detail)
			}
			return nil
		}
		return job.Run(ctx, dreamAgentID)
	},
}

func init() {
	dreamCmd.Flags().StringVar(&dreamAgentID, "agent", "", "agent id to process (required)")
	dreamCmd.Flags().BoolVar(&dreamScan, "scan", false, "run the consistency scan instead of the aging pass")
	dreamCmd.Flags().BoolVar(&dreamRepair, "repair", false, "repair findings from --scan (blob-level only)")
}
