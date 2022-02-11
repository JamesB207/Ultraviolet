package worker

import (
	"log"
	"net"

	"github.com/realDragonium/Ultraviolet/config"
)

type WorkerManager interface {
	AddBackend(domains []string, ch chan<- BackendRequest)
	RemoveBackend(domains []string)
	KnowsDomain(domain string) bool
	Register(worker UpdatableWorker, update bool)
	Start() error
}

func NewWorkerManager(cfg config.UVConfigReader, reqCh <-chan net.Conn) WorkerManager {
	manager := workerManager{
		reqCh:     reqCh,
		cfgReader: cfg,
		domains:   make(map[string]chan<- BackendRequest),
		workers:   []UpdatableWorker{},
	}
	return &manager
}

type workerManager struct {
	cfgReader config.UVConfigReader
	reqCh     <-chan net.Conn
	domains   map[string]chan<- BackendRequest
	workers   []UpdatableWorker
}

func (manager *workerManager) Start() error {
	cfg, err := manager.cfgReader()
	if err != nil {
		return err
	}
	workerCfg := config.NewWorkerConfig(cfg)
	for i := 0; i < cfg.NumberOfWorkers; i++ {
		wrk := NewWorker(workerCfg, manager.reqCh)
		go func(bw BasicWorker) {
			bw.Work()
		}(wrk)
		manager.Register(&wrk, true)
	}
	log.Printf("Running %v worker(s)", cfg.NumberOfWorkers)
	return nil
}

func (manager *workerManager) SetReqChan(reqCh <-chan net.Conn) {
	manager.reqCh = reqCh
}

func (manager *workerManager) AddBackend(domains []string, ch chan<- BackendRequest) {
	for _, domain := range domains {
		manager.domains[domain] = ch
	}
	manager.update()
}

func (manager *workerManager) RemoveBackend(domains []string) {
	for _, domain := range domains {
		delete(manager.domains, domain)
	}
	manager.update()
}

func (manager *workerManager) Register(worker UpdatableWorker, update bool) {
	manager.workers = append(manager.workers, worker)
	if update {
		worker.Update(manager.domains)
	}
}

func (manager *workerManager) update() {
	for _, wrk := range manager.workers {
		wrk.Update(manager.domains)
	}
}

func (manager *workerManager) KnowsDomain(domain string) bool {
	_, knows := manager.domains[domain]
	return knows
}
