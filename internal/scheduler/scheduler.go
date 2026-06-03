package scheduler

import "github.com/robfig/cron/v3"

type Job struct {
	Spec string // cron 表达式,如 "0 0 * * *" 或 "@every 1h"
	Run  func()
}

type Scheduler struct {
	cron *cron.Cron
}

// New 构造调度器并注册所有作业;任一 spec 非法则返回错误。
func New(jobs ...Job) (*Scheduler, error) {
	c := cron.New()
	for _, j := range jobs {
		if _, err := c.AddFunc(j.Spec, j.Run); err != nil {
			return nil, err
		}
	}
	return &Scheduler{cron: c}, nil
}

func (s *Scheduler) Start() { s.cron.Start() }

// Stop 停止调度并等待正在运行的作业结束(cron.Stop() 返回的 ctx 在作业完成时 Done)。
func (s *Scheduler) Stop() { <-s.cron.Stop().Done() }

func (s *Scheduler) EntryCount() int { return len(s.cron.Entries()) }
