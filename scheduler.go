// Package venue 提供活动场地与设备的内存排期能力。
package venue

import "sort"

// Slot 表示活动安排中的一个时段。
// 时间单位为整数毫秒；Start 计入占用，End 不计入。
type Slot struct {
	Start     int64
	End       int64
	Resources []string
}

// Scheduler 在内存中管理各活动的资源占用。
// 仅供单进程顺序调用，非并发安全。
type Scheduler struct {
	// activities 保存每个活动最近一次成功提交的完整安排。
	activities map[string][]Slot
}

// New 创建空的 Scheduler。
func New() *Scheduler {
	return &Scheduler{activities: make(map[string][]Slot)}
}

// Submit 提交活动 activityID 的完整新安排。
// 首次提交即新建，再次提交即整体替换，空安排即释放全部占用。
// 任一冲突则整体拒绝并返回 false，所有已提交安排保持不变。
func (s *Scheduler) Submit(activityID string, slots []Slot) bool {
	if !validSlots(slots) {
		return false
	}
	// 校验新安排与其他活动的占用冲突；该活动的旧安排不参与校验。
	for otherID, otherSlots := range s.activities {
		if otherID == activityID {
			continue
		}
		for _, existing := range otherSlots {
			for _, slot := range slots {
				if conflict(existing, slot) {
					return false
				}
			}
		}
	}
	// 全部校验通过，整体生效。
	if len(slots) == 0 {
		delete(s.activities, activityID)
		return true
	}
	s.activities[activityID] = cloneSlots(slots)
	return true
}

// validSlots 校验时段本身合法且新安排内部无资源冲突。
func validSlots(slots []Slot) bool {
	for _, slot := range slots {
		if slot.Start >= slot.End {
			return false
		}
	}
	for i := 0; i < len(slots); i++ {
		for j := i + 1; j < len(slots); j++ {
			if conflict(slots[i], slots[j]) {
				return false
			}
		}
	}
	return true
}

// conflict 判断两个时段是否因共享资源且时间重叠而冲突。
func conflict(a, b Slot) bool {
	if a.Start >= b.End || b.Start >= a.End {
		return false
	}
	for _, ra := range a.Resources {
		for _, rb := range b.Resources {
			if ra == rb {
				return true
			}
		}
	}
	return false
}

// cloneSlots 深拷贝时段切片，隔离调用方后续修改。
func cloneSlots(slots []Slot) []Slot {
	out := make([]Slot, len(slots))
	for i, slot := range slots {
		resources := make([]string, len(slot.Resources))
		copy(resources, slot.Resources)
		out[i] = Slot{Start: slot.Start, End: slot.End, Resources: resources}
	}
	return out
}

// EarliestCommonGap 返回 resources 全部同时可用的最早开始时刻，
// 要求连续可用时长至少为 duration，且整个占用落在 [from, to] 内。
// 无解时返回 ok=false。查询不预占资源。
func (s *Scheduler) EarliestCommonGap(resources []string, from, to, duration int64) (start int64, ok bool) {
	if duration <= 0 {
		return from, from <= to
	}
	if to-from < duration {
		return 0, false
	}
	wanted := make(map[string]bool, len(resources))
	for _, r := range resources {
		wanted[r] = true
	}
	// 收集涉及目标资源、且与搜索范围重叠的全部占用区间。
	var busy [][2]int64
	for _, slots := range s.activities {
		for _, slot := range slots {
			if slot.End <= from || slot.Start >= to {
				continue
			}
			if !usesAny(slot, wanted) {
				continue
			}
			busy = append(busy, [2]int64{slot.Start, slot.End})
		}
	}
	// 合并占用区间，扫描范围 [from, to] 内的空窗。
	sort.Slice(busy, func(i, j int) bool { return busy[i][0] < busy[j][0] })
	cursor := from
	for _, iv := range busy {
		if iv[0] > cursor && iv[0]-cursor >= duration {
			return cursor, true
		}
		if iv[1] > cursor {
			cursor = iv[1]
		}
	}
	if to-cursor >= duration {
		return cursor, true
	}
	return 0, false
}

// usesAny 判断时段是否占用目标资源集合中的任一资源。
func usesAny(slot Slot, wanted map[string]bool) bool {
	for _, r := range slot.Resources {
		if wanted[r] {
			return true
		}
	}
	return false
}

// Schedule 返回活动最近一次成功提交的完整安排副本。
// 不存在时返回 nil。
func (s *Scheduler) Schedule(activityID string) []Slot {
	slots, ok := s.activities[activityID]
	if !ok {
		return nil
	}
	return cloneSlots(slots)
}
