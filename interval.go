package scheduler

import "sort"

// interval 是半开区间 [start, end)。
type interval struct {
	start int64
	end   int64
}

// overlaps 判断两个半开区间是否重叠；端点相接不算重叠。
func overlaps(a, b interval) bool {
	return a.start < b.end && b.start < a.end
}

// mergeIntervals 合并重叠或相邻的区间，返回按开始时刻排序的结果。
func mergeIntervals(ivs []interval) []interval {
	if len(ivs) == 0 {
		return nil
	}
	sorted := make([]interval, len(ivs))
	copy(sorted, ivs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].start != sorted[j].start {
			return sorted[i].start < sorted[j].start
		}
		return sorted[i].end < sorted[j].end
	})
	merged := []interval{sorted[0]}
	for _, iv := range sorted[1:] {
		last := &merged[len(merged)-1]
		if iv.start <= last.end {
			if iv.end > last.end {
				last.end = iv.end
			}
			continue
		}
		merged = append(merged, iv)
	}
	return merged
}
