package venue

import (
	"reflect"
	"testing"
)

func slotsEqual(a, b []Slot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Start != b[i].Start || a[i].End != b[i].End ||
			!reflect.DeepEqual(a[i].Resources, b[i].Resources) {
			return false
		}
	}
	return true
}

// 新安排最后一个时段冲突时，旧安排仍完整保留。
func TestSubmitConflictKeepsOldSchedule(t *testing.T) {
	s := New()
	if !s.Submit("other", []Slot{{Start: 100, End: 200, Resources: []string{"projector"}}}) {
		t.Fatal("setup submit should succeed")
	}
	old := []Slot{
		{Start: 0, End: 50, Resources: []string{"room-a"}},
		{Start: 50, End: 80, Resources: []string{"room-a", "speaker"}},
	}
	if !s.Submit("act", old) {
		t.Fatal("initial submit should succeed")
	}
	// 前两个时段合法，最后一个时段与 other 的投影仪占用冲突。
	repl := []Slot{
		{Start: 0, End: 50, Resources: []string{"room-a"}},
		{Start: 60, End: 90, Resources: []string{"speaker"}},
		{Start: 150, End: 180, Resources: []string{"projector"}},
	}
	if s.Submit("act", repl) {
		t.Fatal("conflicting submit should be rejected")
	}
	got := s.Schedule("act")
	if !slotsEqual(got, old) {
		t.Fatalf("old schedule not preserved: got %+v", got)
	}
	// 其他活动不受影响。
	if got := s.Schedule("other"); !slotsEqual(got, []Slot{{Start: 100, End: 200, Resources: []string{"projector"}}}) {
		t.Fatalf("other activity changed: got %+v", got)
	}
	// 失败提交不影响之后的查询：投影仪仍只有 other 的 [100,200) 占用。
	if start, ok := s.EarliestCommonGap([]string{"projector"}, 100, 300, 30); !ok || start != 200 {
		t.Fatalf("query after failed submit: got start=%d ok=%v, want 200 true", start, ok)
	}
}

// 多个资源各自有空，却没有满足所需连续时长的共同空窗。
func TestNoCommonGapThoughEachResourceFree(t *testing.T) {
	s := New()
	// 会议室在 [0,100) 被占，投影仪在 [100,200) 被占：
	// 每个资源各自都有 100ms 空窗，但共同空窗为 0。
	if !s.Submit("a", []Slot{{Start: 0, End: 100, Resources: []string{"room"}}}) {
		t.Fatal("submit a failed")
	}
	if !s.Submit("b", []Slot{{Start: 100, End: 200, Resources: []string{"projector"}}}) {
		t.Fatal("submit b failed")
	}
	if start, ok := s.EarliestCommonGap([]string{"room", "projector"}, 0, 200, 10); ok {
		t.Fatalf("expected no common gap, got start=%d", start)
	}
	// 各自单独查询则都有空窗。
	if _, ok := s.EarliestCommonGap([]string{"room"}, 0, 200, 10); !ok {
		t.Fatal("room alone should have a gap")
	}
	if _, ok := s.EarliestCommonGap([]string{"projector"}, 0, 200, 10); !ok {
		t.Fatal("projector alone should have a gap")
	}
}

func TestSubmitReplaceAndRelease(t *testing.T) {
	s := New()
	if !s.Submit("act", []Slot{{Start: 0, End: 100, Resources: []string{"room"}}}) {
		t.Fatal("first submit should succeed")
	}
	// 替换后旧占用释放，其他活动可占用原时段。
	if !s.Submit("act", []Slot{{Start: 200, End: 300, Resources: []string{"room"}}}) {
		t.Fatal("replace should succeed")
	}
	if !s.Submit("other", []Slot{{Start: 0, End: 100, Resources: []string{"room"}}}) {
		t.Fatal("old slot should be free after replace")
	}
	// 空安排释放全部占用。
	if !s.Submit("act", nil) {
		t.Fatal("empty submit should succeed")
	}
	if got := s.Schedule("act"); got != nil {
		t.Fatalf("expected nil schedule after release, got %+v", got)
	}
	if start, ok := s.EarliestCommonGap([]string{"room"}, 150, 400, 100); !ok || start != 150 {
		t.Fatalf("expected gap at 150 after release, got start=%d ok=%v", start, ok)
	}
}

func TestSubmitInternalConflictRejected(t *testing.T) {
	s := New()
	// 同一活动内部两个时段重叠占用同一资源，必须拒绝。
	bad := []Slot{
		{Start: 0, End: 100, Resources: []string{"room"}},
		{Start: 50, End: 150, Resources: []string{"room"}},
	}
	if s.Submit("act", bad) {
		t.Fatal("internal conflict should be rejected")
	}
	if got := s.Schedule("act"); got != nil {
		t.Fatalf("rejected submit must not be stored, got %+v", got)
	}
}

func TestBackToBackSlotsAllowed(t *testing.T) {
	s := New()
	if !s.Submit("a", []Slot{{Start: 0, End: 100, Resources: []string{"room"}}}) {
		t.Fatal("submit a failed")
	}
	// 前一时段结束时后一时段可立即开始。
	if !s.Submit("b", []Slot{{Start: 100, End: 200, Resources: []string{"room"}}}) {
		t.Fatal("back-to-back submit should succeed")
	}
}

func TestEarliestCommonGapBasics(t *testing.T) {
	s := New()
	s.Submit("a", []Slot{{Start: 10, End: 40, Resources: []string{"room", "speaker"}}})
	s.Submit("b", []Slot{{Start: 60, End: 90, Resources: []string{"room"}}})
	// [0,10) 不足 20，最早共同空窗为 [40,60)。
	if start, ok := s.EarliestCommonGap([]string{"room", "speaker"}, 0, 100, 20); !ok || start != 40 {
		t.Fatalf("got start=%d ok=%v, want 40 true", start, ok)
	}
	// 结束时刻等于范围终点合法。
	if start, ok := s.EarliestCommonGap([]string{"speaker"}, 0, 100, 60); !ok || start != 40 {
		t.Fatalf("got start=%d ok=%v, want 40 true", start, ok)
	}
	// 占用必须完整落在范围内。
	if _, ok := s.EarliestCommonGap([]string{"speaker"}, 0, 100, 61); ok {
		t.Fatal("duration exceeding range should have no solution")
	}
	// 查询不预占资源，重复查询结果一致。
	if start, ok := s.EarliestCommonGap([]string{"room", "speaker"}, 0, 100, 20); !ok || start != 40 {
		t.Fatalf("repeat query changed: got start=%d ok=%v", start, ok)
	}
}

func TestScheduleReturnsCopy(t *testing.T) {
	s := New()
	s.Submit("act", []Slot{{Start: 0, End: 100, Resources: []string{"room"}}})
	got := s.Schedule("act")
	got[0].Start = 999
	got[0].Resources[0] = "tampered"
	again := s.Schedule("act")
	if again[0].Start != 0 || again[0].Resources[0] != "room" {
		t.Fatalf("schedule mutated through returned slice: %+v", again)
	}
}
