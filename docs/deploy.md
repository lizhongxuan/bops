# 部署说明

## 构建

```bash
go build -o bin/bops ./cmd/bops
go build -o bin/bops-agent ./cmd/bops-agent
```

## 运行

```bash
./bin/bops plan -f examples/simple.yaml
./bin/bops apply -f examples/simple.yaml
```

本地代理示例:
```bash
./bin/bops-agent -id agent-local
```
