// Package scheduler 提供活动场地与设备的内存排期能力。
//
// 由上层在单进程中顺序调用，不做并发控制。
// 时间统一使用整数毫秒，时段为半开区间 [Start, End)。
package scheduler

// Slot 表示一场活动安排中的一个时段。
// Start 计入占用，End 不计入；Resources 中的资源在该时段内被同时独占。
type Slot struct {
	Start     int64
	End       int64
	Resources []string
}

// Scheduler 保存全部活动的已提交安排。
type Scheduler struct {
	activities map[string][]Slot
}

// New 创建一个空的 Scheduler。
func New() *Scheduler {
	return &Scheduler{activities: make(map[string][]Slot)}
}
