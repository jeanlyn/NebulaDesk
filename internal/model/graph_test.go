package model

import "testing"

func TestSpaceSchemaFieldIndexes(t *testing.T) {
	sc := &SpaceSchema{
		Space: "horae_task_dag",
		Indexes: []IndexSchema{
			{Name: "idx_task_id", Kind: "TAG", Target: "task", Fields: []string{"task_id(32)"}},
			{Name: "idx_task_composite", Kind: "TAG", Target: "task", Fields: []string{`"task_name(64)"`, "`task_priority`"}},
			{Name: "idx_link_active", Kind: "EDGE", Target: "task_link", Fields: []string{"both_active_daily"}},
		},
	}

	m1 := sc.FieldIndexes("TAG", "task", "task_id")
	if len(m1) != 1 || m1[0].IndexName != "idx_task_id" || m1[0].Position != 1 || m1[0].TotalCols != 1 {
		t.Fatalf("FieldIndexes(task_id) = %+v", m1)
	}

	m2 := sc.FieldIndexes("TAG", "task", "task_priority")
	if len(m2) != 1 || m2[0].IndexName != "idx_task_composite" || m2[0].Position != 2 || m2[0].TotalCols != 2 {
		t.Fatalf("FieldIndexes(task_priority) = %+v", m2)
	}

	m3 := sc.FieldIndexes("TAG", "task", "cluster")
	if len(m3) != 0 {
		t.Fatalf("expected no index for cluster, got %+v", m3)
	}

	m4 := sc.FieldIndexes("EDGE", "task_link", "both_active_daily")
	if len(m4) != 1 || m4[0].IndexName != "idx_link_active" {
		t.Fatalf("FieldIndexes(edge both_active_daily) = %+v", m4)
	}
}
