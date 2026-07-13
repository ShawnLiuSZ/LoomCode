# 安全模型

> 权限控制和沙箱机制。

## 权限层级

```
Permission
  ├── Allowlist (白名单)
  │     ├── Shell 命令白名单
  │     ├── 文件路径白名单
  │     └── 敏感文件模式
  └── Gate (门控)
        ├── Review 模式 - 每个编辑需确认
        ├── Auto 模式 - 自动应用
        └── Yolo 模式 - 跳过确认
```

## 默认安全规则

### 敏感文件
以下文件默认需要额外确认：
- `.env`, `.env.local`, `.env.production`
- `credentials`, `secrets`
- `.pem`, `.key`
- `id_rsa`, `id_ed25519`

### 危险命令
以下命令默认被阻止：
- `rm -rf`, `rm -r`
- `sudo`
- `chmod 777`
- `| sh`
- `mkfs.`
- `dd if=`

## 配置

在 `loomcode.toml` 中配置：

```toml
[security]
mode = "review"  # review | auto | yolo
```

---

*此文档为存根页，完整内容待补充。*
