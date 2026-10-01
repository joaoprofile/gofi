# base/cronjob

`import "github.com/gofi-labs/gofi-sdk-go/base/cronjob"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Funções

### cronjob.CheckAllJobsHealth

```go
func CheckAllJobsHealth(jobHandles []*JobHandle)
```

## cronjob.JobGenerator

```go
type JobGenerator[T any] struct {
	GenericObject  []T
	ProcessJobFunc func(T) error
	BatchSize      int
}
```

### cronjob.NewJobGenerator

```go
func NewJobGenerator[T any](genericObject []T, batchSize int, processJobFunc func(T) error) *JobGenerator[T]
```

### JobGenerator.GenerateJobs

```go
func (j *JobGenerator[T]) GenerateJobs() [][]func()
```

### JobGenerator.RunWithPool

```go
func (j *JobGenerator[T]) RunWithPool(pool *WorkerPool)
```

## cronjob.JobHandle

```go
type JobHandle struct {
	// contains filtered or unexported fields
}
```

### cronjob.ScheduleJob

```go
func ScheduleJob(ctx context.Context, cfg ScheduleConfig, job func()) *JobHandle
```

ScheduleJob starts a background job according to cfg and returns a handle to control it.
Panics if cfg is invalid (invalid mode, non-positive interval, out-of-range hour/minute, or unknown location).

### JobHandle.GetStatus

```go
func (j *JobHandle) GetStatus() JobStatus
```

### JobHandle.Stop

```go
func (j *JobHandle) Stop()
```

## cronjob.JobStatus

```go
type JobStatus string
```

```go
const (
	JobRunning JobStatus = "running"
	JobStopped JobStatus = "stopped"
	JobFailed  JobStatus = "failed"
)
```

## cronjob.Locker

```go
type Locker interface {
	// Acquire reports whether this replica won key; the claim expires after ttl.
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
}
```

Locker grants one replica the right to execute a scheduled slot.

## cronjob.RedisLocker

```go
type RedisLocker struct {
	Client goredis.UniversalClient
}
```

RedisLocker claims slots with SET NX and a TTL; claims are never released,
so a slot runs once even if replicas tick at slightly different times.

### RedisLocker.Acquire

```go
func (l RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
```

Acquire implements Locker.

## cronjob.ScheduleConfig

```go
type ScheduleConfig struct {
	Mode         ScheduleMode   // Scheduling mode
	Interval     time.Duration  // Used in 'interval' mode
	Hour         int            // Used in 'fixed' mode
	Minute       int            // Used in 'fixed' mode
	Weekdays     []time.Weekday // If empty, runs every day
	LocationName string         // Optional, if empty, uses the system's timezone

	// Name identifies the job across replicas; required with Locker.
	Name string
	// Locker makes each scheduled run execute on one replica only (see
	// RedisLocker). Nil runs the job on every replica, as before.
	Locker Locker
}
```

## cronjob.ScheduleMode

```go
type ScheduleMode string
```

```go
const (
	Interval ScheduleMode = "interval"
	Fixed    ScheduleMode = "fixed"
)
```

## cronjob.WorkerPool

```go
type WorkerPool struct {
	Workers int
	Jobs    chan []func()
	// contains filtered or unexported fields
}
```

### cronjob.NewPool

```go
func NewPool(workers int) *WorkerPool
```

### WorkerPool.Close

```go
func (p *WorkerPool) Close()
```

### WorkerPool.EnqueueJobBatch

```go
func (p *WorkerPool) EnqueueJobBatch(batch []func())
```

### WorkerPool.Start

```go
func (p *WorkerPool) Start()
```

### WorkerPool.Wait

```go
func (p *WorkerPool) Wait()
```

