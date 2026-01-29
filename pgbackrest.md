# Postgres 增量备份方案设计报告

## 1. 现状分析与核心冲突

### 1.1 现有逻辑分析

* **路径管理**：**“每一次全量备份，我都会创建一个新的文件夹(按照时间戳)”**。
* **执行命令**：强制指定 `--type=full`。



### 1.2 核心冲突 (Blocker)

**pgbackrest 的增量备份严重依赖于固定的仓库路径。**

* **pgbackrest 的工作原理**：它会在 `repo1-path` 下维护一个 `backup.info` 文件和 `backup` 目录。当执行增量备份（Incremental）时，它必须读取 `backup.info` 来找到最近一次有效的“全量备份”或“差异备份”作为基准（Base），只拷贝发生变化的数据块（WAL或Changed Blocks）。
* **当前问题**：如果你每次都按时间戳创建新文件夹（例如 `/data/backup/20231027_1000/`），对于 pgbackrest 来说，这是一个**全新的、空的仓库**。它找不到任何历史备份记录，因此无法进行增量备份，只能再次全量备份。

---

## 2. 方案设计目标

1. **启用增量备份**：大幅减少备份时间和存储空间占用。
2. **自动化保留策略**：利用 pgbackrest 内置功能自动清理过期备份，替代手动管理时间戳文件夹。
3. **兼容性**：在保留现有 Golang 架构的基础上进行最小化改造。

---

## 3. 详细设计方案

### 3.1 目录结构改造 (关键变更)

必须放弃“每次备份创建新时间戳文件夹”的策略，改为**每个实例使用固定的备份根目录**。

* **Before (当前模式 - 不支持增量):**
* `/backups/inst_101/20231027_0100/` (全量)
* `/backups/inst_101/20231028_0100/` (全量 - 视为全新仓库)


* **After (推荐模式 - 支持增量):**
* `/backups/inst_101/` (作为 `repo1-path`)
* `backup/` (pgbackrest 自动管理)
* `20231027-010000F` (Full)
* `20231027-020000I` (Incr)
* `20231027-030000I` (Incr)







**设计决策**：`backupDataPath` 入参应该固定为该实例的专属目录（如 `/data/pgbackrest/inst_{id}`），而不是带时间戳的目录。

### 3.2 备份类型策略

pgbackrest 支持三种类型：

* **Full (全量)**：备份所有文件。是增量备份的基础。
* **Diff (差异)**：备份自上次 *Full* 以来变化的文件。
* **Incr (增量)**：备份自上次 *Full, Diff 或 Incr* 以来变化的文件。

**推荐策略**：

* **每周** 1次 Full。
* **每天** 1次 Diff (可选，加速恢复)。
* **每小时** (或按需) 1次 Incr。

### 3.3 代码改造方案 (Golang)

需要对现有的 `cmd` 拼接逻辑和配置生成逻辑进行微调。

#### A. 修改配置生成调用 (Logic Layer)

在调用 `GenerateBackupPgbackrestConf` 之前，不要生成带时间戳的路径，而是生成固定的实例路径。

```go
// 假设 instID 是实例ID
// [Old] basedir := fmt.Sprintf("/data/backups/%d/%s", instID, time.Now().Format("20060102150405"))
// [New] 路径固定，pgbackrest 会在内部通过哈希和时间戳区分备份
basedir := fmt.Sprintf("/data/backups/%d", instID) 

// 调用配置生成 (保持不变，但传入的 backupDataPath 已经是固定的了)
confContent := GenerateBackupPgbackrestConf(hostType, ..., basedir, ...)

```

#### B. 修改 Command 执行逻辑

我们需要根据调度策略动态传入备份类型。

```go
// 定义备份类型枚举
type BackupType string
const (
    BackupTypeFull BackupType = "full"
    BackupTypeDiff BackupType = "diff"
    BackupTypeIncr BackupType = "incr"
)

// 封装执行命令的方法
func GetBackupCommand(instID int, cpuNum int, bType BackupType) string {
    // 注意：这里去掉了 --config 中的动态时间戳路径，假设 config 放在固定的位置或者由外层控制
    // 建议 config 文件也固定位置，例如 /etc/pgbackrest/{instID}/pgbackrest.conf
    
    cmd := fmt.Sprintf(
        "pgbackrest --stanza=paf --log-level-console=debug --config=/etc/pgbackrest/%d/pgbackrest.conf --type=%s backup --exclude=log/ --process-max=%d", 
        instID, 
        bType, // 动态传入 full, diff, 或 incr
        cpuNum,
    )
    return cmd
}

```

### 3.4 配置文件调整 (`GenerateBackupPgbackrestConf`)

目前的配置模板中设置了 `repo1-retention-full=10`。这很好，但配合增量备份，建议明确保留策略。

**建议更新后的模板片段**：

```ini
[global]
repo1-path=%s
log-path=%s
start-fast=y

# 保留策略：保留最近 10 个全量备份及其关联的增量/差异备份
repo1-retention-full=10 
repo1-retention-full-type=count # 建议用 count 更直观，或保持 time

# 自动清理：当全量备份过期被删除时，依附于它的增量备份也会自动被清理
# 这样你就不用自己写代码去删文件夹了

```

---

## 4. 实施后的工作流模拟

假设你采用了固定路径 `/data/backups/1001`：

1. **Day 1 (Full)**:
* 执行命令: `... --type=full ...`
* 结果: pgbackrest 在 `/data/backups/1001/backup` 下创建一个全量备份目录。


2. **Day 1 + 1小时 (Incr)**:
* 执行命令: `... --type=incr ...`
* 结果: pgbackrest 检测到已有 Full，创建一个很小的文件夹，只包含变动的数据块。


3. **Day 1 + 2小时 (Incr)**:
* 执行命令: `... --type=incr ...`
* 结果: 基于上一次 Incr 进行备份。


4. **Day 10 (过期清理)**:
* 执行命令: `... --type=full ...`
* 结果: 创建新的 Full。pgbackrest 检查 `repo1-retention-full=10`，如果超过限制，它会自动删除最老的 Full 及其下属的所有 Incr 备份。



---

## 5. 风险与注意事项

### 5.1 首次执行

当你从“时间戳文件夹模式”切换到“固定目录模式”时，第一次执行 `incr` 会发生什么？

* 如果固定目录下没有任何备份，pgbackrest 会自动将 `incr` 降级为 `full`。这是安全的特性。

### 5.2 归档 (WAL Archiving)

增量备份虽然可以备份数据文件，但为了能够**恢复到任意时间点 (PITR)**，通常强烈建议配置 WAL 归档。

* **现状**: 你的配置中没有看到 `archive-push` 相关设置。
* **建议**: 如果只做简单的增量备份（恢复到备份时刻），当前配置够用。如果需要恢复到“故障发生的具体那一秒”，需要在 `postgresql.conf` 中配置 `archive_command` 指向 pgbackrest。

### 5.3 锁与并发

* 代码中用到了 `path.Clean` 和 `fmt.Sprintf`，这是线程安全的。
* **必须确保**: 对同一个 `stanza` (同一个实例)，不能同时运行两个 `backup` 命令。pgbackrest 会利用锁文件 (`.lock`) 自动阻止这种情况，但业务层最好也加个状态锁。

---

## 6. 总结与下一步建议

### 结论

要实现增量备份，**核心必须改变“每次创建新文件夹”的习惯**。pgbackrest 自身具备极强的版本管理和过期清理能力，应当信任工具本身。

### Next Steps for You (我能为你做的)

1. **修改代码逻辑**：如果你愿意，我可以为你重写 `GenerateBackupPgbackrestConf` 和外层的调用逻辑，将路径处理改为固定路径模式。
2. **制定定时任务逻辑**：如果你需要 Golang 代码来判断“今天是周几，该跑Full还是Incr”，我可以提供这部分逻辑代码。
3. **恢复脚本设计**：增量备份的恢复（Restore）不需要人工选择所有的增量文件，pgbackrest 会自动计算。如果你需要恢复命令的封装代码，我也可以提供。

你希望先从哪一步开始？