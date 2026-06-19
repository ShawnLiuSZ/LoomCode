# 工具执行引擎

> 工具调用的并行与串行机制。

## 架构

```
Agent
  └── Executor
        ├── Registry (工具注册中心)
        ├── Guards (执行守卫)
        └── Parallel/Serial 调度
```

## 执行策略

| 工具类型 | 执行方式 | 原因 |
|---------|---------|------|
| 只读工具 (read, grep, glob) | 并行 | 无副作用，可并发 |
| 写入工具 (write, edit, bash) | 串行 | 有副作用，需顺序执行 |

## 守卫机制

通过 `AddGuard()` 添加执行前检查：

```go
executor.AddGuard(func(tc Call) error {
    // 检查权限
    return nil // 或返回 error 阻止执行
})
```

## 并发控制

- 默认最大并行数：3
- 可通过 `SetMaxParallel()` 调整
- 使用信号量控制并发

---

*此文档为存根页，完整内容待补充。*
