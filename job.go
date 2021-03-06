package gocron

import (
	"fmt"
	"sync"
	"time"
)

// Job struct stores the information necessary to run a Job
type Job struct {
	sync.RWMutex
	interval          int                      // pause interval * unit between runs
	duration          time.Duration            // time duration between runs
	unit              timeUnit                 // time units, ,e.g. 'minutes', 'hours'...
	startsImmediately bool                     // if the Job should run upon scheduler start
	jobFunc           string                   // the Job jobFunc to run, func[jobFunc]
	atTime            time.Duration            // optional time at which this Job runs when interval is day
	startAtTime       time.Time                // optional time at which the Job starts
	err               error                    // error related to Job
	lastRun           time.Time                // datetime of last run
	nextRun           time.Time                // datetime of next run
	scheduledWeekday  *time.Weekday            // Specific day of the week to start on
	dayOfTheMonth     int                      // Specific day of the month to run the job
	funcs             map[string]interface{}   // Map for the function task store
	fparams           map[string][]interface{} // Map for function and params of function
	tags              []string                 // allow the user to tag Jobs with certain labels
	runConfig         runConfig                // configuration for how many times to run the job
	runCount          int                      // number of times the job ran
	timer             *time.Timer
}

type runConfig struct {
	finiteRuns         bool
	maxRuns            int
	removeAfterLastRun bool
}

// NewJob creates a new Job with the provided interval
func NewJob(interval int) *Job {
	return &Job{
		interval:          interval,
		lastRun:           time.Time{},
		nextRun:           time.Time{},
		funcs:             make(map[string]interface{}),
		fparams:           make(map[string][]interface{}),
		tags:              []string{},
		startsImmediately: true,
	}
}

// Run the Job and immediately reschedule it
func (j *Job) run() {
	j.Lock()
	defer j.Unlock()
	j.runCount++
	go func() {
		_, _ = callJobFuncWithParams(j.funcs[j.jobFunc], j.fparams[j.jobFunc])
	}()
}

func (j *Job) neverRan() bool {
	j.RLock()
	defer j.RUnlock()
	return j.lastRun.IsZero()
}

func (j *Job) getStartsImmediately() bool {
	j.RLock()
	defer j.RUnlock()
	return j.startsImmediately
}

func (j *Job) setStartsImmediately(b bool) {
	j.Lock()
	defer j.Unlock()
	j.startsImmediately = b
}

func (j *Job) setTimer(t *time.Timer) {
	j.Lock()
	defer j.Unlock()
	j.timer = t
}

func (j *Job) getAtTime() time.Duration {
	j.RLock()
	defer j.RUnlock()
	return j.atTime
}

func (j *Job) setAtTime(t time.Duration) {
	j.Lock()
	defer j.Unlock()
	j.atTime = t
}

func (j *Job) getStartAtTime() time.Time {
	j.RLock()
	defer j.RUnlock()
	return j.startAtTime
}

func (j *Job) setStartAtTime(t time.Time) {
	j.Lock()
	defer j.Unlock()
	j.startAtTime = t
}

// Err returns an error if one occurred while creating the Job
func (j *Job) Err() error {
	j.RLock()
	defer j.RUnlock()
	return j.err
}

// Tag allows you to add arbitrary labels to a Job that do not
// impact the functionality of the Job
func (j *Job) Tag(tags ...string) {
	j.Lock()
	defer j.Unlock()
	j.tags = append(j.tags, tags...)
}

// Untag removes a tag from a Job
func (j *Job) Untag(t string) {
	j.Lock()
	defer j.Unlock()
	var newTags []string
	for _, tag := range j.tags {
		if t != tag {
			newTags = append(newTags, tag)
		}
	}

	j.tags = newTags
}

// Tags returns the tags attached to the Job
func (j *Job) Tags() []string {
	j.RLock()
	defer j.RUnlock()
	return j.tags
}

// ScheduledTime returns the time of the Job's next scheduled run
func (j *Job) ScheduledTime() time.Time {
	j.RLock()
	defer j.RUnlock()
	return j.nextRun
}

// ScheduledAtTime returns the specific time of day the Job will run at
func (j *Job) ScheduledAtTime() string {
	j.RLock()
	defer j.RUnlock()
	return fmt.Sprintf("%d:%d", j.atTime/time.Hour, (j.atTime%time.Hour)/time.Minute)
}

// Weekday returns which day of the week the Job will run on and
// will return an error if the Job is not scheduled weekly
func (j *Job) Weekday() (time.Weekday, error) {
	j.RLock()
	defer j.RUnlock()
	if j.scheduledWeekday == nil {
		return time.Sunday, ErrNotScheduledWeekday
	}
	return *j.scheduledWeekday, nil
}

// LimitRunsTo limits the number of executions of this job to n.
// The job will remain in the scheduler.
// Note: If a job is added to a running scheduler and this method is used
// you may see the job run more than the set limit as job is scheduled immediately
// by default upon being added to the scheduler. It is recommended to use the
// LimitRunsTo() func on the scheduler chain when scheduling the job.
// For example: scheduler.LimitRunsTo(1).Do()
func (j *Job) LimitRunsTo(n int) {
	j.Lock()
	defer j.Unlock()
	j.runConfig = runConfig{
		finiteRuns: true,
		maxRuns:    n,
	}
}

// RemoveAfterLastRun sets the job to be removed after it's last run (when limited)
func (j *Job) RemoveAfterLastRun() *Job {
	j.Lock()
	defer j.Unlock()
	j.runConfig.removeAfterLastRun = true
	return j
}

func (j *Job) getRemoveAfterLastRun() bool {
	j.RLock()
	defer j.RUnlock()
	return j.runConfig.removeAfterLastRun
}

// shouldRun evaluates if this job should run again
// based on the runConfig
func (j *Job) shouldRun() bool {
	j.RLock()
	defer j.RUnlock()
	return !j.runConfig.finiteRuns || j.runCount < j.runConfig.maxRuns
}

// LastRun returns the time the job was run last
func (j *Job) LastRun() time.Time {
	j.RLock()
	defer j.RUnlock()
	return j.lastRun
}

func (j *Job) setLastRun(t time.Time) {
	j.Lock()
	defer j.Unlock()
	j.lastRun = t
}

// NextRun returns the time the job will run next
func (j *Job) NextRun() time.Time {
	j.RLock()
	defer j.RUnlock()
	return j.nextRun
}

func (j *Job) setNextRun(t time.Time) {
	j.Lock()
	defer j.Unlock()
	j.nextRun = t
}

// RunCount returns the number of time the job ran so far
func (j *Job) RunCount() int {
	j.RLock()
	defer j.RUnlock()
	return j.runCount
}

func (j *Job) stopTimer() {
	j.Lock()
	defer j.Unlock()
	if j.timer != nil {
		j.timer.Stop()
	}
}
