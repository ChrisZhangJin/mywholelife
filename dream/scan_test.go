package dream

import (
	"context"
	"testing"
	"time"

	"mywholelife/store"
)

func kindsOf(rep store.ScanReport) map[string]bool {
	m := map[string]bool{}
	for _, f := range rep.Findings {
		m[f.Kind] = true
	}
	return m
}

func TestScanCleanStoreZero(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-ok")
	base := time.Now().Unix()

	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "h"}, Cfg: FromEnv(), Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, repair := range []bool{false, true} {
		rep, err := j.Scan(ctx, ag, repair)
		if err != nil {
			t.Fatalf("Scan(repair=%v): %v", repair, err)
		}
		if len(rep.Findings) != 0 {
			t.Fatalf("clean store must report zero (repair=%v), got %+v", repair, rep.Findings)
		}
	}
}

func TestScanReportsButDestroysNothing(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-ok")
	base := time.Now().Unix()
	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "h"}, Cfg: FromEnv(), Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run: %v", err)
	}

	orphan := "agents/" + ag + "/projects/20260101-orphan.tar.zst"
	if err := blobs.PutFolder(ctx, orphan, []byte("norow")); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}
	idx, _ := st.GetIndex(ctx, ag)
	stale := append(append([]byte{}, idx...), []byte("- ghost | h | 20260101-ghost\n")...)
	if err := st.PutIndex(ctx, ag, stale); err != nil {
		t.Fatalf("inject stale line: %v", err)
	}

	rep, err := j.Scan(ctx, ag, false)
	if err != nil {
		t.Fatalf("Scan report: %v", err)
	}
	k := kindsOf(rep)
	if !k["orphan_blob"] || !k["index_mismatch"] {
		t.Fatalf("report mode must surface orphan+stale findings: %+v", rep.Findings)
	}
	if ok, _ := blobs.Exists(ctx, orphan); !ok {
		t.Fatal("report mode must not delete the orphan blob")
	}
}

func TestScanRepairRemovesOrphanAndTornTar(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-live")
	base := time.Now().Unix()
	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "h"}, Cfg: FromEnv(), Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run: %v", err)
	}

	orphan := "agents/" + ag + "/projects/20260101-orphan.tar.zst"
	if err := blobs.PutFolder(ctx, orphan, []byte("norow")); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}
	// Torn compress: the live memory keeps its .tar.zst but a stale .tar lingers.
	tornTar := "agents/" + ag + "/projects/20260101-live.tar"
	if err := blobs.PutFolder(ctx, tornTar, []byte("stale tar")); err != nil {
		t.Fatalf("seed torn tar: %v", err)
	}

	rep, err := j.Scan(ctx, ag, true)
	if err != nil {
		t.Fatalf("Scan repair: %v", err)
	}
	if k := kindsOf(rep); !k["orphan_blob"] || !k["torn_compress"] {
		t.Fatalf("repair scan must detect orphan+torn: %+v", rep.Findings)
	}
	if ok, _ := blobs.Exists(ctx, orphan); ok {
		t.Fatal("repair mode must delete the orphan blob")
	}
	if ok, _ := blobs.Exists(ctx, tornTar); ok {
		t.Fatal("repair mode must delete the stale torn .tar")
	}
	// The live compressed blob must survive.
	if ok, _ := blobs.Exists(ctx, "agents/"+ag+"/projects/20260101-live.tar.zst"); !ok {
		t.Fatal("repair must not touch the live .tar.zst")
	}
}
