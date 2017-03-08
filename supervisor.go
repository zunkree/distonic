package distonic

import (
	"container/list"
	"log"
	"sync"

	git "github.com/libgit2/git2go"
	"github.com/spf13/viper"
)

type Supervisor struct {
	repos   map[string]*Watcher
	workers []*Worker
	queue   *list.List
	bell    chan bool
}

type Order struct {
	repoName   string
	repo       *git.Repository
	branchName string
	commit     *git.Commit
}

func NewSupervisor() (*Supervisor, error) {
	var err error
	reposConfig := viper.Sub("repos")
	numWorkers := viper.GetInt("num_workers")

	s := &Supervisor{
		repos:   map[string]*Watcher{},
		workers: []*Worker{},
		queue:   list.New(),
		bell:    make(chan bool, 1)}

	for repoName := range viper.GetStringMap("repos") {
		repoSettings := reposConfig.Sub(repoName)
		s.repos[repoName], err = NewWatcher(
			repoName,
			repoSettings.GetString("url"),
			repoSettings.GetStringSlice("branches"))
		if err != nil {
			log.Printf("Cannot create watcher for repo `%s`: %s", repoName, err)
			return nil, err
		}
		log.Printf("Created watcher for repo: %s", repoName)
	}

	for n := 0; n < numWorkers; n++ {
		w, err := NewWorker()
		if err != nil {
			log.Printf("Cannot create worker #%s: %s", n, err)
			return nil, err
		}
		s.workers = append(s.workers, w)
	}

	return s, nil
}

func (s *Supervisor) Run() {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		s.runWorkers()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		s.runWatchers()
		wg.Done()
	}()

	wg.Wait()
}

func (s *Supervisor) runWatchers() {
	orders := make(chan *Order, len(s.repos))

	for name, watcher := range s.repos {
		go func() {
			err := watcher.Run(orders)
			if err != nil {
				log.Printf("Error in watcher for repo %s: %s", name, err)
			}
		}()
	}

	for order := range orders {
		s.schedule(order)
	}
}

func (s *Supervisor) runWorkers() {
	orders := make(chan *Order, len(s.workers))

	for _, worker := range s.workers {
		go func() {
			worker.Run(orders)
		}()
	}

	for _ = range s.bell {
		log.Printf("Bell rings")
		for s.queue.Len() > 0 {
			order := s.queue.Remove(s.queue.Front())
			orders <- order.(*Order)
		}
	}
}

func (s *Supervisor) schedule(order *Order) error {
	s.queue.PushBack(order)

	select {
	case <-s.bell:
		log.Print("Silenced the bell")
	default:
		log.Print("Bell is idle")
	}
	s.bell <- true
	log.Print("Rang the bell")
	return nil
}
