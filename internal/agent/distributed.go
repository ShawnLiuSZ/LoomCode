package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// TaskStatus 任务状态
type TaskStatus int

const (
	// TaskPending 等待执行
	TaskPending TaskStatus = iota
	// TaskRunning 执行中
	TaskRunning
	// TaskCompleted 已完成
	TaskCompleted
	// TaskFailed 失败
	TaskFailed
	// TaskCancelled 已取消
	TaskCancelled
)

// String 返回状态字符串
func (s TaskStatus) String() string {
	switch s {
	case TaskPending:
		return "pending"
	case TaskRunning:
		return "running"
	case TaskCompleted:
		return "completed"
	case TaskFailed:
		return "failed"
	case TaskCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Task 分布式任务
type Task struct {
	ID       string
	Name     string
	Input    string
	Output   string
	Status   TaskStatus
	Error    error
	AgentID  string
	Created  time.Time
	Started  time.Time
	Finished time.Time
}

// Worker 工作节点
type Worker struct {
	ID       string
	Agent    *Agent
	mu       sync.RWMutex
	busy     bool
	tasks    []*Task
	completed int
	failed   int
}

// NewWorker 创建工作节点
func NewWorker(id string, agent *Agent) *Worker {
	return &Worker{
		ID:    id,
		Agent: agent,
	}
}

// IsBusy 是否忙碌
func (w *Worker) IsBusy() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.busy
}

// Execute 执行任务
func (w *Worker) Execute(ctx context.Context, task *Task) error {
	w.mu.Lock()
	w.busy = true
	task.Status = TaskRunning
	task.AgentID = w.ID
	task.Started = time.Now()
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.busy = false
		w.mu.Unlock()
	}()

	result, err := w.Agent.Run(ctx, task.Input)

	w.mu.Lock()
	defer w.mu.Unlock()

	task.Finished = time.Now()

	if err != nil {
		task.Status = TaskFailed
		task.Error = err
		w.failed++
		return err
	}

	task.Status = TaskCompleted
	task.Output = result
	w.completed++
	return nil
}

// Stats 返回工作节点统计
func (w *Worker) Stats() (completed, failed int) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.completed, w.failed
}

// Scheduler 任务调度器
type Scheduler struct {
	mu      sync.RWMutex
	workers []*Worker
	tasks   []*Task
	queue   chan *Task
	muTask  sync.Mutex
}

// NewScheduler 创建调度器
func NewScheduler(workers []*Worker) *Scheduler {
	s := &Scheduler{
		workers: workers,
		tasks:   make([]*Task, 0),
		queue:   make(chan *Task, 100),
	}

	// 启动调度协程
	go s.schedule()

	return s
}

// Submit 提交任务
func (s *Scheduler) Submit(name, input string) *Task {
	task := &Task{
		ID:      fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Name:    name,
		Input:   input,
		Status:  TaskPending,
		Created: time.Now(),
	}

	s.muTask.Lock()
	s.tasks = append(s.tasks, task)
	s.muTask.Unlock()

	s.queue <- task

	return task
}

// schedule 调度任务
func (s *Scheduler) schedule() {
	for task := range s.queue {
		// 等待可用的工作节点
		worker := s.selectWorker()

		// 异步执行
		go func(w *Worker, t *Task) {
			w.Execute(context.Background(), t)
		}(worker, task)
	}
}

// selectWorker 选择可用的工作节点
func (s *Scheduler) selectWorker() *Worker {
	for {
		s.mu.RLock()
		for _, w := range s.workers {
			if !w.IsBusy() {
				s.mu.RUnlock()
				return w
			}
		}
		s.mu.RUnlock()

		// 等待一段时间后重试
		time.Sleep(100 * time.Millisecond)
	}
}

// GetTask 获取任务状态
func (s *Scheduler) GetTask(id string) *Task {
	s.muTask.Lock()
	defer s.muTask.Unlock()

	for _, task := range s.tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

// GetTasks 获取所有任务
func (s *Scheduler) GetTasks() []*Task {
	s.muTask.Lock()
	defer s.muTask.Unlock()

	result := make([]*Task, len(s.tasks))
	copy(result, s.tasks)
	return result
}

// GetPendingTasks 获取待执行任务
func (s *Scheduler) GetPendingTasks() []*Task {
	s.muTask.Lock()
	defer s.muTask.Unlock()

	var result []*Task
	for _, task := range s.tasks {
		if task.Status == TaskPending {
			result = append(result, task)
		}
	}
	return result
}

// GetRunningTasks 获取执行中任务
func (s *Scheduler) GetRunningTasks() []*Task {
	s.muTask.Lock()
	defer s.muTask.Unlock()

	var result []*Task
	for _, task := range s.tasks {
		if task.Status == TaskRunning {
			result = append(result, task)
		}
	}
	return result
}

// GetCompletedTasks 获取已完成任务
func (s *Scheduler) GetCompletedTasks() []*Task {
	s.muTask.Lock()
	defer s.muTask.Unlock()

	var result []*Task
	for _, task := range s.tasks {
		if task.Status == TaskCompleted {
			result = append(result, task)
		}
	}
	return result
}

// GetStats 获取调度器统计
func (s *Scheduler) GetStats() (pending, running, completed, failed int) {
	s.muTask.Lock()
	defer s.muTask.Unlock()

	for _, task := range s.tasks {
		switch task.Status {
		case TaskPending:
			pending++
		case TaskRunning:
			running++
		case TaskCompleted:
			completed++
		case TaskFailed:
			failed++
		}
	}
	return
}

// Cancel 取消任务
func (s *Scheduler) Cancel(id string) bool {
	s.muTask.Lock()
	defer s.muTask.Unlock()

	for _, task := range s.tasks {
		if task.ID == id && task.Status == TaskPending {
			task.Status = TaskCancelled
			task.Finished = time.Now()
			return true
		}
	}
	return false
}

// Close 关闭调度器
func (s *Scheduler) Close() {
	close(s.queue)
}

// DistributedAgent 分布式 Agent 协作管理器
type DistributedAgent struct {
	scheduler *Scheduler
	workers   []*Worker
}

// NewDistributedAgent 创建分布式 Agent
func NewDistributedAgent(providers []provider.Provider, tools *tool.Registry, workerCount int) *DistributedAgent {
	workers := make([]*Worker, workerCount)
	for i := 0; i < workerCount; i++ {
		agent := New(providers[i%len(providers)], tools)
		workers[i] = NewWorker(fmt.Sprintf("worker_%d", i), agent)
	}

	return &DistributedAgent{
		scheduler: NewScheduler(workers),
		workers:   workers,
	}
}

// Submit 提交任务
func (d *DistributedAgent) Submit(name, input string) *Task {
	return d.scheduler.Submit(name, input)
}

// GetTask 获取任务状态
func (d *DistributedAgent) GetTask(id string) *Task {
	return d.scheduler.GetTask(id)
}

// GetStats 获取统计信息
func (d *DistributedAgent) GetStats() (pending, running, completed, failed int) {
	return d.scheduler.GetStats()
}

// Wait 等待所有任务完成
func (d *DistributedAgent) Wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for tasks")
		}

		pending, running, _, _ := d.GetStats()
		if pending == 0 && running == 0 {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// Close 关闭分布式 Agent
func (d *DistributedAgent) Close() {
	d.scheduler.Close()
}
