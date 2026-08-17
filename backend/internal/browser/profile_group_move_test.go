package browser

import (
	"testing"

	"ant-chrome/backend/internal/database"
	_ "modernc.org/sqlite"
)

// 回归:MoveInstancesToGroup 必须同步内存 m.Profiles,否则 List()(读内存)返回旧 group_id,
// 按分组筛选查不到数据(线上 bug:分组下拉有计数、筛选却为空)。
func TestMoveInstancesToGroupSyncsMemoryAndDB(t *testing.T) {
	db, err := database.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	m := NewManager(nil, t.TempDir())
	m.ProfileDAO = NewSQLiteProfileDAO(db.GetConn())

	p1 := &Profile{ProfileId: "p1", ProfileName: "e1"}
	p2 := &Profile{ProfileId: "p2", ProfileName: "e2"}
	for _, p := range []*Profile{p1, p2} {
		m.Profiles[p.ProfileId] = p
		if err := m.ProfileDAO.Upsert(p); err != nil {
			t.Fatalf("Upsert %s: %v", p.ProfileId, err)
		}
	}

	if err := m.MoveInstancesToGroup([]string{"p1"}, "g1"); err != nil {
		t.Fatalf("MoveInstancesToGroup: %v", err)
	}

	// 内存同步(List 读内存,这是 bug 所在)。
	if m.Profiles["p1"].GroupId != "g1" {
		t.Fatalf("内存未同步: p1.GroupId=%q want g1", m.Profiles["p1"].GroupId)
	}
	if m.Profiles["p2"].GroupId != "" {
		t.Fatalf("p2 不应被移动: GroupId=%q", m.Profiles["p2"].GroupId)
	}

	// DB 落库。
	got, err := m.ProfileDAO.GetById("p1")
	if err != nil {
		t.Fatalf("GetById: %v", err)
	}
	if got.GroupId != "g1" {
		t.Fatalf("DB 未落库: GroupId=%q want g1", got.GroupId)
	}

	// 移出分组:groupId="" 同样同步内存。
	if err := m.MoveInstancesToGroup([]string{"p1"}, ""); err != nil {
		t.Fatalf("MoveInstancesToGroup ungroup: %v", err)
	}
	if m.Profiles["p1"].GroupId != "" {
		t.Fatalf("移出分组内存未同步: GroupId=%q", m.Profiles["p1"].GroupId)
	}
}
