package cmd

import (
	"fmt"

	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/workers"
	"github.com/amirdaaee/Glide/internals/workers/download"
	"github.com/amirdaaee/Glide/internals/workers/ingest"
	"github.com/amirdaaee/Glide/internals/workers/upload"
	"github.com/spf13/cobra"
)

var workerStep string

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start a stateless pipeline worker",
	RunE: func(cmd *cobra.Command, args []string) error {
		step := domain.JobStep(workerStep)
		h, err := stepHandler(step)
		if err != nil {
			return err
		}
		p := wire.GetProvider()
		return p.Invoke(func(sub pipeline.ISubscriber, pub pipeline.IPublisher) error {
			return workers.Run(cmd.Context(), sub, pub, step, h)
		})
	},
}

func stepHandler(step domain.JobStep) (workers.IStepHandler, error) {
	switch step {
	case domain.JobStepIngest:
		return ingest.New(), nil
	case domain.JobStepDownload:
		return download.New(), nil
	case domain.JobStepUpload:
		return upload.New(), nil
	default:
		return nil, fmt.Errorf("unknown worker step %q (want ingest, download, or upload)", step)
	}
}

func init() {
	workerCmd.Flags().StringVar(&workerStep, "step", "", "pipeline step to consume (ingest|download|upload)")
	_ = workerCmd.MarkFlagRequired("step")
	rootCmd.AddCommand(workerCmd)
}
