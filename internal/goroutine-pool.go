package internal

import (
	"fmt"
	"sync"
)

type Task func() error

type Pool struct {
	workerNum int
	tasks     chan Task
	wg        sync.WaitGroup
	quit      chan struct{}
}

func NewPool(workerNum int) *Pool {
	return &Pool{
		workerNum: workerNum,
		tasks:     make(chan Task),
		quit:      make(chan struct{}),
	}
}

func (p *Pool) Run() {
	for i := 0; i < p.workerNum; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.tasks:
			if task != nil {
				if err := task(); err != nil {
					fmt.Println("任务执行失败:", err)
				}
			}
		case <-p.quit:
			return
		}
	}
}

func (p *Pool) AddTask(task Task) {
	select {
	case p.tasks <- task:
	case <-p.quit:
	}
}

func (p *Pool) Shutdown() {
	close(p.quit)
	p.wg.Wait()
	close(p.tasks)
}
