package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewJobsKeepsFiftyAndMarksFilesGone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	var saved []*Job
	for i := 0; i < 60; i++ {
		saved = append(saved, &Job{ID: string(rune('a' + i%26)), Status: "ok", Files: []File{{Name: "x.pdf", Size: 1}}})
	}
	data, _ := json.Marshal(saved)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	j := NewJobs(path)
	list := j.List()
	if len(list) != keepJobs {
		t.Fatalf("%d jobs kept, want %d", len(list), keepJobs)
	}
	if f := list[0].Files[0]; !f.Gone {
		t.Errorf("file after a restart not gone: %+v", f)
	}
	if _, ok := j.File(list[0].ID, "x.pdf"); ok {
		t.Error("a gone file is served")
	}
}

func TestClearKeepsTheRunningJob(t *testing.T) {
	j := NewJobs(filepath.Join(t.TempDir(), "jobs.json"))
	j.jobs = []*Job{{ID: "a", Status: "ok"}, {ID: "b", Status: "running"}}
	j.running = j.jobs[1]
	j.Clear()
	if list := j.List(); len(list) != 1 || list[0].ID != "b" {
		t.Errorf("after Clear: %+v", list)
	}
}
