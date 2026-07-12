package worker

type IWorkerPool interface {
	GetNextWorker() IWorker
}
