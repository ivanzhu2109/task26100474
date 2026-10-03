# venue — 活动场地与设备排期库

Go 实现的内存排期库，供上层在单进程中顺序调用（非并发安全）。
提供三项能力：整场安排提交、共同空窗查询、当前安排读取。

## 数据模型

```go
type Slot struct {
    Start     int64    // 开始时刻（毫秒，计入占用）
    End       int64    // 结束时刻（毫秒，不计入占用）
    Resources []string // 必须同时占用的独占资源标识
}
```

活动与资源均使用上层提供的稳定标识（`string`），库不管理其资料。
时段之间允许任意空隙；前一时段结束时后一时段可立即开始。

## API

```go
s := venue.New()

// 提交活动的完整新安排：首次即新建，再次即整体替换，空切片即释放全部占用。
// 校验时排除本活动旧安排，但检查其他活动及新安排内部冲突。
// 任一时段冲突则整体拒绝（返回 false），所有已提交安排保持不变。
ok := s.Submit("activity-1", []venue.Slot{
    {Start: 1000, End: 2000, Resources: []string{"room-a", "projector"}},
    {Start: 3000, End: 4000, Resources: []string{"room-a", "speaker"}},
})

// 共同空窗查询：返回全部资源同时可用、连续时长 >= duration 的最早开始时刻，
// 整个占用须落在 [from, to] 内（结束等于 to 合法）；无解时 ok=false。
// 查询以全部已提交安排为准，不预占资源。
start, ok := s.EarliestCommonGap([]string{"room-a", "projector"}, 0, 10000, 500)

// 读取活动最近一次成功提交的完整安排（副本），不存在时返回 nil。
slots := s.Schedule("activity-1")
```

## 测试

```sh
go test ./...
```

覆盖场景：冲突时旧安排完整保留、多资源各自有空但无共同空窗、
替换与释放、活动内部冲突拒绝、首尾相接时段、空窗边界与查询不预占、读取副本隔离。
