package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Job is one run of a module from the web interface: what it was, how it went, and what it said.
type Job struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`  // travel-expense, apple-invoice, …
	Title   string    `json:"title"` // "Reisekosten 12/2026 senden"
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitzero"`
	Status  string    `json:"status"` // running, ok, failed
	Error   string    `json:"error,omitempty"`
	Output  []string  `json:"output"`
	Files   []File    `json:"files,omitempty"`
}

// File is a document a job made, kept in memory for downloading until the server stops.
type File struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	data []byte
}

// work is what a job does: it logs through the module loggers, and may hand back files.
type work func() ([]File, error)

// Jobs runs one job at a time — the modules share their loggers and a browser — and keeps the
// latest ones, their files only until a restart.
type Jobs struct {
	mu      sync.Mutex
	run     sync.Mutex
	path    string // where the history is kept, without files
	jobs    []*Job // oldest first
	running *Job
}

const keepJobs = 100

var errBusy = errors.New("another job is running: wait for it to finish")

func NewJobs(path string) *Jobs {
	j := &Jobs{path: path}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &j.jobs)
		for _, job := range j.jobs {
			if job.Status == "running" { // the server stopped while it ran
				job.Status, job.Error = "failed", "interrupted"
			}
			job.Files = nil // kept in memory only: gone with the restart
		}
	}
	return j
}

// Start runs w in the background as a new job, unless one is running.
func (j *Jobs) Start(kind, title string, w work) (*Job, error) {
	if !j.run.TryLock() {
		return nil, errBusy
	}
	job := &Job{ID: newJobID(), Kind: kind, Title: title, Started: time.Now(), Status: "running", Output: []string{}}
	j.mu.Lock()
	j.jobs = append(j.jobs, job)
	if len(j.jobs) > keepJobs {
		j.jobs = j.jobs[len(j.jobs)-keepJobs:]
	}
	j.running = job
	j.mu.Unlock()

	go func() {
		defer j.run.Unlock()
		lines := &lineWriter{add: func(line string) {
			j.mu.Lock()
			job.Output = append(job.Output, line)
			j.mu.Unlock()
		}}
		setModuleLogs(io.MultiWriter(logOut, lines))
		files, err := safely(w)
		lines.Flush()
		setModuleLogs(logOut)

		j.mu.Lock()
		job.Ended, job.Files, job.Status = time.Now(), files, "ok"
		if err != nil {
			job.Status, job.Error = "failed", err.Error()
		}
		j.running = nil
		j.mu.Unlock()
		j.save()
	}()
	return job, nil
}

// safely runs w, turning a panic into an error: one module's bug is not the server's end.
func safely(w work) (files []File, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return w()
}

// List is the jobs, newest first, without their output.
func (j *Jobs) List() []Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	list := make([]Job, 0, len(j.jobs))
	for i := len(j.jobs) - 1; i >= 0; i-- {
		job := *j.jobs[i]
		job.Output = nil
		list = append(list, job)
	}
	return list
}

// Get is a copy of one job, with its output.
func (j *Jobs) Get(id string) (Job, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, job := range j.jobs {
		if job.ID == id {
			copied := *job
			copied.Output = append([]string(nil), job.Output...)
			return copied, true
		}
	}
	return Job{}, false
}

// File is a file a job made, if it is still in memory.
func (j *Jobs) File(id, name string) (File, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, job := range j.jobs {
		if job.ID != id {
			continue
		}
		for _, f := range job.Files {
			if f.Name == name && f.data != nil {
				return f, true
			}
		}
	}
	return File{}, false
}

// Running is the job running now, if any.
func (j *Jobs) Running() *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.running == nil {
		return nil
	}
	copied := *j.running
	copied.Output = nil
	return &copied
}

func (j *Jobs) save() {
	j.mu.Lock()
	data, err := json.MarshalIndent(j.jobs, "", "  ")
	j.mu.Unlock()
	if err == nil {
		_ = os.WriteFile(j.path, data, 0o600)
	}
}

func newJobID() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// lineWriter hands on what is written to it line by line.
type lineWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
	add func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line) // not a whole line yet
			return len(p), nil
		}
		w.add(strings.TrimRight(line, "\r\n"))
	}
}

// Flush hands on a last line without its newline.
func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if rest := strings.TrimSpace(w.buf.String()); rest != "" {
		w.add(rest)
	}
	w.buf.Reset()
}
