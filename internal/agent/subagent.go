package agent

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ShawnLiuSZ/loomcode/internal/provider"
	"github.com/ShawnLiuSZ/loomcode/internal/tool"
)

// maxSubAgentDepth 子 Agent 递归 spawn 的最大深度，防止成本/栈爆炸。
const maxSubAgentDepth = 3
const maxAgents = 100

// SubAgent 子 Agent
type SubAgent struct {
	ID       string
	Name     string
	Role     string
	ParentID string
	Depth    int

	agent      *Agent
	status     SubAgentStatus
	result     string
	err        error
	mu         sync.Mutex
	done       chan struct{}
	started    bool
	finishedAt time.Time
	ctx        context.Context
	cancel     context.CancelFunc
	bus        *MessageBus
	msgCh      <-chan BusMessage
}

// SubAgentStatus 子 Agent 状态
type SubAgentStatus int

const (
	StatusPending SubAgentStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

func (s SubAgentStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// SubAgentManager 子 Agent 管理器
type SubAgentManager struct {
	mu        sync.RWMutex
	agents    map[string]*SubAgent
	idCounter int
	bus       *MessageBus
}

// NewSubAgentManager 创建子 Agent 管理器
func NewSubAgentManager() *SubAgentManager {
	return &SubAgentManager{
		agents: make(map[string]*SubAgent),
	}
}

// spawn 是唯一的创建入口：原子设置 ParentID/Depth 并强制深度上限兜底。
// depth 越界（<0 或 > maxSubAgentDepth）时返回 nil 且不注册——确保深度限制无法被任何路径绕过。
func (m *SubAgentManager) spawn(name, role, parentID string, depth int, p provider.Provider, registry *tool.Registry) *SubAgent {
	if depth < 0 || depth > maxSubAgentDepth {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.idCounter++
	id := fmt.Sprintf("sub_%d", m.idCounter)

	ctx, cancel := context.WithCancel(context.Background())
	sa := &SubAgent{
		ID:       id,
		Name:     name,
		Role:     role,
		ParentID: parentID,
		Depth:    depth,
		agent:    New(p, registry),
		status:   StatusPending,
		done:     make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
		bus:      m.bus,
	}

	// 立即订阅消息总线，确保 agent 运行期间的消息不会丢失
	if m.bus != nil {
		sa.msgCh = m.bus.Subscribe(id)
	}

	m.agents[id] = sa
	return sa
}

// Spawn 创建并启动一个【顶层】子 Agent（depth 0，无父）。
// 子 Agent 内部如需再 spawn，必须用 SpawnChild，以受 maxSubAgentDepth 约束、防止无限递归。
func (m *SubAgentManager) Spawn(name, role string, p provider.Provider, registry *tool.Registry) *SubAgent {
	return m.spawn(name, role, "", 0, p, registry)
}

// SpawnChild 创建一个子级 Agent，设置 ParentID/Depth 并校验递归深度。
// 超过 maxSubAgentDepth 时返回错误且不注册，防止子 agent 无限递归 spawn。
func (m *SubAgentManager) SpawnChild(parent *SubAgent, name, role string, p provider.Provider, registry *tool.Registry) (*SubAgent, error) {
	depth := parent.Depth + 1
	sa := m.spawn(name, role, parent.ID, depth, p, registry)
	if sa == nil {
		return nil, fmt.Errorf("max sub-agent depth %d exceeded", maxSubAgentDepth)
	}
	return sa, nil
}

// Get 获取子 Agent
func (m *SubAgentManager) Get(id string) (*SubAgent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sa, ok := m.agents[id]
	return sa, ok
}

// List 列出所有子 Agent
func (m *SubAgentManager) List() []*SubAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SubAgent, 0, len(m.agents))
	for _, sa := range m.agents {
		result = append(result, sa)
	}
	return result
}

// Cancel 取消子 Agent
func (m *SubAgentManager) Cancel(id string) error {
	m.mu.RLock()
	sa, ok := m.agents[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("sub-agent %q not found", id)
	}

	sa.cancel()
	return nil
}

// CancelAll 取消所有子 Agent
func (m *SubAgentManager) CancelAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, sa := range m.agents {
		sa.cancel()
	}
}

func (m *SubAgentManager) SetBus(bus *MessageBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bus = bus
}

func (m *SubAgentManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for id, sa := range m.agents {
		sa.mu.Lock()
		done := sa.status == StatusCompleted || sa.status == StatusFailed || sa.status == StatusCancelled
		finishedAt := sa.finishedAt
		sa.mu.Unlock()

		if done && finishedAt.Before(cutoff) {
			delete(m.agents, id)
		}
	}

	if len(m.agents) > maxAgents {
		type entry struct {
			id         string
			finishedAt time.Time
		}
		var completed []entry
		for id, sa := range m.agents {
			sa.mu.Lock()
			done := sa.status == StatusCompleted || sa.status == StatusFailed || sa.status == StatusCancelled
			fa := sa.finishedAt
			sa.mu.Unlock()
			if done {
				completed = append(completed, entry{id: id, finishedAt: fa})
			}
		}
		sort.Slice(completed, func(i, j int) bool {
			return completed[i].finishedAt.Before(completed[j].finishedAt)
		})
		excess := len(m.agents) - maxAgents
		if excess > len(completed) {
			excess = len(completed)
		}
		for i := 0; i < excess; i++ {
			delete(m.agents, completed[i].id)
		}
	}
}

// Run 启动子 Agent 执行任务。
// 一次性：仅在 Pending 状态启动；完成/失败/取消/运行中再次调用都是 no-op，
// 避免对已关闭的 done channel 二次 close 而 panic（H11）。
func (sa *SubAgent) Run(task string) {
	sa.mu.Lock()
	if sa.status != StatusPending {
		sa.mu.Unlock()
		return
	}
	sa.status = StatusRunning
	sa.started = true
	sa.mu.Unlock()

	go func() {
		defer close(sa.done)

		result, err := sa.agent.Run(sa.ctx, task)

		sa.mu.Lock()
		defer sa.mu.Unlock()

		if err != nil {
			if sa.ctx.Err() != nil {
				sa.status = StatusCancelled
				sa.err = sa.ctx.Err()
			} else {
				sa.status = StatusFailed
				sa.err = err
			}
			sa.finishedAt = time.Now()
			return
		}

		sa.status = StatusCompleted
		sa.result = result
		sa.finishedAt = time.Now()
	}()
}

// RunParallel 并行运行多个子 Agent
func (m *SubAgentManager) RunParallel(tasks map[string]string) map[string]string {
	var wg sync.WaitGroup
	results := make(map[string]string)
	var mu sync.Mutex

	for id, task := range tasks {
		sa, ok := m.Get(id)
		if !ok {
			continue
		}

		wg.Add(1)
		go func(sa *SubAgent, task string) {
			defer wg.Done()
			sa.Run(task)
			sa.Wait()

			mu.Lock()
			results[sa.ID] = sa.Result()
			mu.Unlock()
		}(sa, task)
	}

	wg.Wait()

	if m.bus != nil {
		// 必须在锁保护下快照 agents：RunParallel 内部会调用 m.Cleanup()，
		// 而 spawn/Cleanup 都会在 m.mu 下修改 m.agents（map 并发读写会 panic）。
		m.mu.RLock()
		snapshot := make([]*SubAgent, 0, len(m.agents))
		for _, sa := range m.agents {
			snapshot = append(snapshot, sa)
		}
		m.mu.RUnlock()

		var allMsgs []BusMessage
		for _, sa := range snapshot {
			ch := sa.Messages()
			if ch == nil {
				continue
			}
			for {
				select {
				case msg, ok := <-ch:
					if !ok {
						goto next
					}
					allMsgs = append(allMsgs, msg)
				default:
					goto next
				}
			}
		next:
		}
		if len(allMsgs) > 0 {
			encoded := ""
			for i, msg := range allMsgs {
				if i > 0 {
					encoded += "|"
				}
				encoded += msg.FromID + "->" + msg.ToID + ":" + msg.Content
			}
			results["_messages"] = encoded
		}
	}

	m.Cleanup()
	return results
}

// Wait 等待子 Agent 完成
// 若 Run 是 no-op（非 Pending 状态，goroutine 未启动），done 永远不会被 close，
// 直接阻塞会导致死锁。已启程则等待 done；未启程则立即返回。
func (sa *SubAgent) Wait() {
	sa.mu.Lock()
	started := sa.started
	sa.mu.Unlock()
	if !started {
		return
	}
	<-sa.done
}

// WaitTimeout 等待子 Agent 完成（带超时）
func (sa *SubAgent) WaitTimeout(timeout time.Duration) error {
	select {
	case <-sa.done:
		return nil
	case <-time.After(timeout):
		sa.cancel()
		return fmt.Errorf("sub-agent %q timed out after %v", sa.ID, timeout)
	}
}

// Status 返回状态
func (sa *SubAgent) Status() SubAgentStatus {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	return sa.status
}

// Result 返回结果
func (sa *SubAgent) Result() string {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	if sa.err != nil {
		return fmt.Sprintf("error: %v", sa.err)
	}
	return sa.result
}

// Error 返回错误
func (sa *SubAgent) Error() error {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	return sa.err
}

// SetMaxSteps 设置最大步数
func (sa *SubAgent) SetMaxSteps(n int) {
	sa.agent.SetMaxSteps(n)
}

func (sa *SubAgent) SetWorkDir(d string) {
	sa.agent.SetWorkDir(d)
}

func (sa *SubAgent) SetHooks(hm *tool.HookManager) {
	sa.agent.SetHooks(hm)
}

func (sa *SubAgent) Send(msg BusMessage) {
	if sa.bus == nil {
		return
	}
	msg.FromID = sa.ID
	sa.bus.Send(msg)
}

func (sa *SubAgent) Messages() <-chan BusMessage {
	if sa.bus == nil {
		return nil
	}
	if sa.msgCh == nil {
		sa.msgCh = sa.bus.Subscribe(sa.ID)
	}
	return sa.msgCh
}
