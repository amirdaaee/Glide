package cmd

import (
	"fmt"

	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/amirdaaee/Glide/internals/workers"
	"github.com/amirdaaee/Glide/internals/workers/download"
	"github.com/amirdaaee/Glide/internals/workers/ingest"
	"github.com/spf13/cobra"
)

var workerStep string

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start a stateless pipeline worker",
	RunE: func(cmd *cobra.Command, args []string) error {
		step := domain.JobStep(workerStep)
		p := wire.GetProvider()
		switch step {
		case domain.JobStepIngest:
			return p.Invoke(func(sub pipeline.ISubscriber, pub pipeline.IPublisher, wPool worker.IWorkerPool, ingestWorker *ingest.Worker) error {
				if err := wPool.Start(cmd.Context()); err != nil {
					return err
				}
				return workers.Run(cmd.Context(), sub, pub, step, ingestWorker)
			})
		case domain.JobStepDownload:
			return p.Invoke(func(sub pipeline.ISubscriber, pub pipeline.IPublisher, wPool worker.IWorkerPool, downloadWorker *download.Worker) error {
				if err := wPool.Start(cmd.Context()); err != nil {
					return err
				}
				return workers.Run(cmd.Context(), sub, pub, step, downloadWorker)
			})
		default:
			return fmt.Errorf("unknown worker step %q (want ingest or download)", step)
		}
	},
}

func init() {
	workerCmd.Flags().StringVar(&workerStep, "step", "", "pipeline step to consume (ingest|download)")
	_ = workerCmd.MarkFlagRequired("step")
	rootCmd.AddCommand(workerCmd)
}
