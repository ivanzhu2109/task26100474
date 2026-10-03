package scheduler

import (
	"errors"
	"fmt"
)

// ErrInvalidSlot 表示时段本身不合法（如开始不早于结束）。
var ErrInvalidSlot = errors.New("scheduler: invalid slot")

// ConflictError 表示新安排与已有占用或自身内部存在冲突。
type ConflictError struct {
	Resource string
	Start    int64
	End      int64
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("scheduler: resource %q conflict in [%d, %d)", e.Resource, e.Start, e.End)
}

// Submit 提交一场活动的完整新安排。
// 首次提交即新建，再次提交即替换，提交空安排即释放该活动的全部占用。
// 验证时排除该活动的旧安排；任一时段冲突则整体不生效，旧安排完整保留。
func (s *Scheduler) Submit(activityID string, slots []Slot) error {
	normalized := make([]Slot, len(slots))
	for i, slot := range slots {
		if slot.Start >= slot.End {
			return fmt.Errorf("%w: start %d must be before end %d", ErrInvalidSlot, slot.Start, slot.End)
		}
		normalized[i] = Slot{
			Start:     slot.Start,
			End:       slot.End,
			Resources: dedupe(slot.Resources),
		}
	}

	// 收集其他活动的占用，按资源分组；该活动的旧安排不参与验证。
	busy := make(map[string][]interval)
	for id, existing := range s.activities {
		if id == activityID {
			continue
		}
		for _, slot := range existing {
			for _, res := range slot.Resources {
				busy[res] = append(busy[res], interval{slot.Start, slot.End})
			}
		}
	}

	// 逐个时段验证：先对其他活动，再对新安排内部已接受的时段。
	for _, slot := range normalized {
		iv := interval{slot.Start, slot.End}
		for _, res := range slot.Resources {
			for _, other := range busy[res] {
				if overlaps(iv, other) {
					return &ConflictError{Resource: res, Start: slot.Start, End: slot.End}
				}
			}
		}
		for _, res := range slot.Resources {
			busy[res] = append(busy[res], iv)
		}
	}

	if len(normalized) == 0 {
		delete(s.activities, activityID)
		return nil
	}
	s.activities[activityID] = normalized
	return nil
}

// FindEarliestWindow 返回全部资源同时可用的最早开始时刻。
// 占用区间为 [start, start+duration)，必须完整落在 [rangeStart, rangeEnd] 内，
// 结束时刻等于 rangeEnd 合法。无解时 ok 为 false。查询不预占资源。
func (s *Scheduler) FindEarliestWindow(resources []string, rangeStart, rangeEnd, duration int64) (start int64, ok bool) {
	resources = dedupe(resources)
	if len(resources) == 0 || duration <= 0 || rangeStart < 0 || rangeEnd < rangeStart {
		return 0, false
	}
	if duration > rangeEnd-rangeStart {
		return 0, false
	}

	wanted := make(map[string]bool, len(resources))
	for _, res := range resources {
		wanted[res] = true
	}
	var ivs []interval
	for _, slots := range s.activities {
		for _, slot := range slots {
			for _, res := range slot.Resources {
				if wanted[res] {
					ivs = append(ivs, interval{slot.Start, slot.End})
				}
			}
		}
	}

	cursor := rangeStart
	for _, iv := range mergeIntervals(ivs) {
		if iv.end <= cursor {
			continue
		}
		if iv.start-cursor >= duration {
			return cursor, true
		}
		if iv.end > cursor {
			cursor = iv.end
		}
	}
	if rangeEnd-cursor >= duration {
		return cursor, true
	}
	return 0, false
}

// Get 返回活动最近一次成功提交的完整安排；不存在时返回 nil。
// 返回值为副本，调用方修改不影响内部状态。
func (s *Scheduler) Get(activityID string) []Slot {
	slots, exists := s.activities[activityID]
	if !exists {
		return nil
	}
	out := make([]Slot, len(slots))
	for i, slot := range slots {
		res := make([]string, len(slot.Resources))
		copy(res, slot.Resources)
		out[i] = Slot{Start: slot.Start, End: slot.End, Resources: res}
	}
	return out
}

func dedupe(resources []string) []string {
	seen := make(map[string]bool, len(resources))
	out := make([]string, 0, len(resources))
	for _, res := range resources {
		if !seen[res] {
			seen[res] = true
			out = append(out, res)
		}
	}
	return out
}
