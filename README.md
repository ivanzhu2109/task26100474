# venue-scheduler

活动场地与设备排期库（Go）。数据保存在内存中，由上层在单进程中顺序调用，不做并发控制。

## 概念

- 一场活动的安排由多个 `Slot` 时段组成，每个时段指定开始、结束（整数毫秒，半开区间 `[Start, End)`）和一组必须同时独占的资源。
- 活动与资源均使用上层提供的稳定字符串标识，本库不管理其资料。
- 时段为半开区间：前一时段结束时，后一时段可立即开始；时段之间允许任意空隙。

## 三项能力

```go
s := scheduler.New()

// 1. 整场安排提交：首次新建，再次替换，空切片释放全部占用。
//    验证时排除该活动的旧安排；任一时段冲突则整体不生效，旧安排完整保留。
err := s.Submit("event-1", []scheduler.Slot{
    {Start: 0, End: 100, Resources: []string{"room-a", "projector"}},
    {Start: 200, End: 300, Resources: []string{"room-a"}},
})
var conflict *scheduler.ConflictError
if errors.As(err, &conflict) {
    // 冲突的资源与时段：conflict.Resource / conflict.Start / conflict.End
}

// 2. 共同空窗查询：返回全部资源同时可用的最早开始时刻。
//    占用 [start, start+duration) 必须落在 [rangeStart, rangeEnd] 内；
//    结束等于 rangeEnd 合法；ok 为 false 表示无解。查询不预占资源。
start, ok := s.FindEarliestWindow([]string{"room-a", "projector"}, 0, 1000, 60)

// 3. 读取当前安排：最近一次成功提交的完整内容（副本），不存在时返回 nil。
slots := s.Get("event-1")
```

## 错误

- `ErrInvalidSlot`：时段开始不早于结束。
- `*ConflictError`：与其他活动或新安排内部的占用冲突，可用 `errors.As` 提取详情。

## 测试

```sh
go test ./...
```
