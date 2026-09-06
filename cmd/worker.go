package cmd

import (
	"fmt"

	"github.com/amirdaaee/Glide/cmd/wire"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/amirdaaee/Glide/internals/workers"
	"github.com/amirdaaee/Glide/internals/workers/download"
	"github.com/amirdaaee/Glide/internals/workers/ingest"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var workerStep string

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start a stateless pipeline worker",
	RunE: func(cmd *cobra.Command, args []string) error {
		ll := log.GetLogger(log.CMD).Named("worker").With(zap.String("step", workerStep))
		step := domain.JobStep(workerStep)
		p := wire.GetProvider()
		var err error
		switch step {
		case domain.JobStepIngest:
			ll.Info("starting ingest worker")
			err = p.Invoke(func(sub pipeline.ISubscriber, pub pipeline.IPublisher, wPool worker.IWorkerPool, ingestWorker *ingest.Worker) error {
				if err := wPool.Start(cmd.Context()); err != nil {
					return err
				}
				return workers.Run(cmd.Context(), sub, pub, step, ingestWorker)
			})
		case domain.JobStepDownload:
			ll.Info("starting download worker")
			err = p.Invoke(func(sub pipeline.ISubscriber, pub pipeline.IPublisher, wPool worker.IWorkerPool, downloadWorker *download.Worker) error {
				if err := wPool.Start(cmd.Context()); err != nil {
					return err
				}
				return workers.Run(cmd.Context(), sub, pub, step, downloadWorker)
			})
		default:
			err = fmt.Errorf("unknown worker step %q (want ingest or download)", step)
		}
		if err != nil && cmd.Context().Err() == nil {
			ll.Error("exited with error", zap.Error(err))
			return err
		}
		ll.Info("stopped")
		return err
	},
}

func init() {
	workerCmd.Flags().StringVar(&workerStep, "step", "", "pipeline step to consume (ingest|download)")
	_ = workerCmd.MarkFlagRequired("step")
	rootCmd.AddCommand(workerCmd)
}
