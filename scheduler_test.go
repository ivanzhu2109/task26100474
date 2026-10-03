package scheduler

import (
	"errors"
	"reflect"
	"testing"
)

func TestSubmitAndGet(t *testing.T) {
	s := New()
	slots := []Slot{
		{Start: 0, End: 100, Resources: []string{"room-a", "projector"}},
		{Start: 200, End: 300, Resources: []string{"room-a"}},
	}
	if err := s.Submit("event-1", slots); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	got := s.Get("event-1")
	if !reflect.DeepEqual(got, slots) {
		t.Fatalf("Get = %+v, want %+v", got, slots)
	}
	// 返回副本，修改不影响内部状态。
	got[0].Resources[0] = "tampered"
	if s.Get("event-1")[0].Resources[0] != "room-a" {
		t.Fatal("Get 返回的不是副本")
	}
	if s.Get("missing") != nil {
		t.Fatal("未提交的活动应返回 nil")
	}
}

func TestSubmitReplaceExcludesOwnOldSlots(t *testing.T) {
	s := New()
	if err := s.Submit("event-1", []Slot{{Start: 0, End: 100, Resources: []string{"room-a"}}}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// 新安排与旧安排重叠，但替换时应排除旧安排。
	replacement := []Slot{{Start: 50, End: 150, Resources: []string{"room-a"}}}
	if err := s.Submit("event-1", replacement); err != nil {
		t.Fatalf("替换不应与自身旧安排冲突: %v", err)
	}
	if got := s.Get("event-1"); !reflect.DeepEqual(got, replacement) {
		t.Fatalf("Get = %+v, want %+v", got, replacement)
	}
}

func TestSubmitEmptyReleasesActivity(t *testing.T) {
	s := New()
	if err := s.Submit("event-1", []Slot{{Start: 0, End: 100, Resources: []string{"room-a"}}}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := s.Submit("event-1", nil); err != nil {
		t.Fatalf("空提交: %v", err)
	}
	if got := s.Get("event-1"); got != nil {
		t.Fatalf("释放后 Get = %+v, want nil", got)
	}
	// 资源已释放，其他活动可以占用同一时段。
	if err := s.Submit("event-2", []Slot{{Start: 0, End: 100, Resources: []string{"room-a"}}}); err != nil {
		t.Fatalf("释放后其他活动应可占用: %v", err)
	}
}

// 场景一：新安排最后一个时段冲突时，旧安排仍完整保留。
func TestFailedSubmitKeepsOldSchedule(t *testing.T) {
	s := New()
	if err := s.Submit("other", []Slot{{Start: 500, End: 600, Resources: []string{"speaker"}}}); err != nil {
		t.Fatalf("Submit other: %v", err)
	}
	old := []Slot{
		{Start: 0, End: 100, Resources: []string{"room-a", "projector"}},
		{Start: 200, End: 300, Resources: []string{"room-a"}},
	}
	if err := s.Submit("event-1", old); err != nil {
		t.Fatalf("Submit old: %v", err)
	}

	// 前两个时段合法，最后一个时段与 other 冲突。
	attempt := []Slot{
		{Start: 0, End: 100, Resources: []string{"room-a"}},
		{Start: 400, End: 450, Resources: []string{"projector"}},
		{Start: 550, End: 650, Resources: []string{"speaker"}},
	}
	err := s.Submit("event-1", attempt)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want *ConflictError", err)
	}
	if conflict.Resource != "speaker" {
		t.Fatalf("冲突资源 = %q, want speaker", conflict.Resource)
	}

	// 旧安排完整保留，未混入被拒绝的内容。
	if got := s.Get("event-1"); !reflect.DeepEqual(got, old) {
		t.Fatalf("失败后 Get = %+v, want %+v", got, old)
	}
	// 其他活动不受影响。
	if got := s.Get("other"); len(got) != 1 || got[0].Start != 500 {
		t.Fatalf("other 的安排被影响: %+v", got)
	}
	// 失败提交不预占资源：attempt 中本可生效的时段仍可被查询到空窗。
	start, ok := s.FindEarliestWindow([]string{"projector"}, 400, 500, 50)
	if !ok || start != 400 {
		t.Fatalf("FindEarliestWindow = (%d, %v), want (400, true)", start, ok)
	}
}

func TestInternalConflictRejected(t *testing.T) {
	s := New()
	err := s.Submit("event-1", []Slot{
		{Start: 0, End: 100, Resources: []string{"room-a"}},
		{Start: 50, End: 150, Resources: []string{"room-a"}},
	})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want *ConflictError", err)
	}
	if got := s.Get("event-1"); got != nil {
		t.Fatalf("内部冲突的提交不应生效, Get = %+v", got)
	}
}

func TestAdjacentSlotsDoNotConflict(t *testing.T) {
	s := New()
	if err := s.Submit("event-1", []Slot{{Start: 0, End: 100, Resources: []string{"room-a"}}}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// 前一时段结束时后一时段可立即开始。
	if err := s.Submit("event-2", []Slot{{Start: 100, End: 200, Resources: []string{"room-a"}}}); err != nil {
		t.Fatalf("相邻时段不应冲突: %v", err)
	}
}

func TestInvalidSlotRejected(t *testing.T) {
	s := New()
	for _, slot := range []Slot{
		{Start: 100, End: 100, Resources: []string{"room-a"}},
		{Start: 200, End: 100, Resources: []string{"room-a"}},
	} {
		if err := s.Submit("event-1", []Slot{slot}); !errors.Is(err, ErrInvalidSlot) {
			t.Fatalf("Submit(%+v) err = %v, want ErrInvalidSlot", slot, err)
		}
	}
}

func TestFindEarliestWindow(t *testing.T) {
	s := New()
	if err := s.Submit("event-1", []Slot{
		{Start: 100, End: 200, Resources: []string{"room-a", "projector"}},
		{Start: 300, End: 400, Resources: []string{"room-a"}},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := s.Submit("event-2", []Slot{
		{Start: 200, End: 300, Resources: []string{"projector"}},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	cases := []struct {
		name                      string
		resources                 []string
		rangeStart, rangeEnd, dur int64
		wantStart                 int64
		wantOK                    bool
	}{
		{"范围起点即可用", []string{"room-a"}, 0, 100, 100, 0, true},
		{"跳过占用段", []string{"room-a"}, 100, 500, 100, 200, true},
		{"结束等于范围终点合法", []string{"room-a"}, 100, 300, 100, 200, true},
		{"多资源取共同空窗", []string{"room-a", "projector"}, 100, 500, 100, 400, true},
		{"无关资源不影响", []string{"speaker"}, 0, 50, 50, 0, true},
		{"范围内放不下", []string{"room-a"}, 100, 400, 150, 0, false},
		{"时长超过范围", []string{"room-a"}, 0, 100, 200, 0, false},
		{"空资源集合", nil, 0, 100, 50, 0, false},
		{"非正时长", []string{"room-a"}, 0, 100, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, ok := s.FindEarliestWindow(tc.resources, tc.rangeStart, tc.rangeEnd, tc.dur)
			if ok != tc.wantOK || (ok && start != tc.wantStart) {
				t.Fatalf("FindEarliestWindow = (%d, %v), want (%d, %v)", start, ok, tc.wantStart, tc.wantOK)
			}
		})
	}
}

// 场景二：多个资源各自有空，却没有满足所需连续时长的共同空窗。
func TestFindEarliestWindowNoCommonGap(t *testing.T) {
	s := New()
	// 会议室在 [0,60) 被占，投影仪在 [60,120) 被占：
	// 各自都有 60ms 的空，但没有任何 60ms 是两者同时空闲的。
	if err := s.Submit("event-1", []Slot{{Start: 0, End: 60, Resources: []string{"room-a"}}}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := s.Submit("event-2", []Slot{{Start: 60, End: 120, Resources: []string{"projector"}}}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if _, ok := s.FindEarliestWindow([]string{"room-a", "projector"}, 0, 120, 60); ok {
		t.Fatal("交错占用下不应存在 60ms 共同空窗")
	}
	// 各自单独查询都有解。
	if start, ok := s.FindEarliestWindow([]string{"room-a"}, 0, 120, 60); !ok || start != 60 {
		t.Fatalf("room-a = (%d, %v), want (60, true)", start, ok)
	}
	if start, ok := s.FindEarliestWindow([]string{"projector"}, 0, 120, 60); !ok || start != 0 {
		t.Fatalf("projector = (%d, %v), want (0, true)", start, ok)
	}
	// 缩短所需时长到 1ms 仍无解（空窗长度恰为 0）；放宽范围则有解。
	if _, ok := s.FindEarliestWindow([]string{"room-a", "projector"}, 0, 120, 1); ok {
		t.Fatal("共同空窗长度为 0，1ms 也应无解")
	}
	if start, ok := s.FindEarliestWindow([]string{"room-a", "projector"}, 0, 180, 60); !ok || start != 120 {
		t.Fatalf("放宽范围 = (%d, %v), want (120, true)", start, ok)
	}
}
