package dao

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestAgentHistoryCapacity is an opt-in, synthetic DAO baseline, not a live load test.
// Timing is reported rather than asserted so slower CI machines remain valid.
func TestAgentHistoryCapacity(t *testing.T) {
	if os.Getenv("LIAISON_TEST_HISTORY_CAPACITY") != "1" {
		t.Skip("set LIAISON_TEST_HISTORY_CAPACITY=1 to run the isolated capacity baseline")
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "capacity.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	// Match the production SQLite connection and PRAGMA settings.
	for _, statement := range []string{"PRAGMA synchronous = NORMAL", "PRAGMA journal_mode = WAL", "PRAGMA cache_size = -2000", "PRAGMA temp_store = MEMORY", "PRAGMA locking_mode = NORMAL", "PRAGMA mmap_size = 268435456"} {
		require.NoError(t, db.Exec(statement).Error)
	}
	require.NoError(t, db.AutoMigrate(&model.AgentApplication{}, &model.AgentAccess{}, &model.EdgeAgentHistory{}, &model.EdgeAgentHistoryPage{}))
	d := &dao{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	require.NoError(t, d.SaveAgentAccess(ctx, &model.AgentAccess{ID: "capacity-access", OwnerID: 1, EdgeID: 1}, true))
	payload := bytes.Repeat([]byte("synthetic-data--"), 1100)[:16<<10]
	histories := make([]model.EdgeAgentHistory, 500)
	for i := range histories {
		histories[i] = model.EdgeAgentHistory{OwnerID: 1, AccessID: "capacity-access", EdgeID: 1, SessionID: fmt.Sprintf("session-%03d", i), Revision: 1, Payload: payload, UpdatedAt: time.Now()}
	}
	require.NoError(t, db.CreateInBatches(histories, 50).Error)
	pages := make([]model.EdgeAgentHistoryPage, 2000)
	for i := range pages {
		pages[i] = model.EdgeAgentHistoryPage{OwnerID: 1, AccessID: "capacity-access", EdgeID: 1, SessionID: histories[i/20].SessionID, Window: uint64(i % 20), Revision: 1, Payload: payload}
	}
	require.NoError(t, db.CreateInBatches(pages, 50).Error)
	t.Logf("environment=%s/%s go=%s cpus=%d sessions=500 pages=2000 payload_bytes=%d", runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumCPU(), len(payload))
	for _, concurrency := range []int{1, 8, 32} {
		t.Run(fmt.Sprintf("workers-%d", concurrency), func(t *testing.T) {
			type sample struct {
				kind    string
				elapsed time.Duration
				err     error
			}
			const operations = 1000
			results := make(chan sample, concurrency*operations)
			start := make(chan struct{})
			var workers sync.WaitGroup
			before := sqlDB.Stats()
			for worker := 0; worker < concurrency; worker++ {
				workers.Add(1)
				go func(worker int) {
					defer workers.Done()
					scope := histories[worker]
					<-start
					for op := 0; op < operations; op++ {
						began := time.Now()
						kind := "pages"
						var operationErr error
						switch op % 20 {
						case 0:
							kind = "list"
							rows, total, err := d.ListEdgeAgentHistories(ctx, &scope, 1)
							operationErr = err
							if err == nil && (len(rows) != 50 || total != 500) {
								operationErr = fmt.Errorf("unexpected history list bounds")
							}
						case 1, 2, 3, 4:
							kind = "write"
							scope.Revision = uint64(concurrency*operations + op + 2)
							scope.UpdatedAt = time.Now()
							operationErr = d.SaveEdgeAgentHistory(ctx, &scope)
						default:
							rows, err := d.ListEdgeAgentHistoryPages(ctx, &scope, 20, 20, 2<<20)
							operationErr = err
							if err == nil && len(rows) != 20 {
								operationErr = fmt.Errorf("unexpected history page bounds")
							}
						}
						results <- sample{kind: kind, elapsed: time.Since(began), err: operationErr}
					}
				}(worker)
			}
			began := time.Now()
			close(start)
			workers.Wait()
			elapsed := time.Since(began)
			close(results)
			durations := map[string][]time.Duration{}
			failures := 0
			for result := range results {
				durations[result.kind] = append(durations[result.kind], result.elapsed)
				if result.err != nil {
					failures++
					t.Errorf("%s failed: %v", result.kind, result.err)
				}
			}
			after := sqlDB.Stats()
			t.Logf("operations=%d elapsed=%s ops_per_second=%.1f errors=%d pool_wait_count=%d pool_wait_total=%s", concurrency*operations, elapsed, float64(concurrency*operations)/elapsed.Seconds(), failures, after.WaitCount-before.WaitCount, after.WaitDuration-before.WaitDuration)
			for _, kind := range []string{"pages", "list", "write"} {
				values := durations[kind]
				sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
				percentile := func(p int) time.Duration { return values[(len(values)*p+99)/100-1] }
				t.Logf("%s n=%d p50=%s p95=%s p99=%s", kind, len(values), percentile(50), percentile(95), percentile(99))
			}
			for worker := 0; worker < concurrency; worker++ {
				row, err := d.GetEdgeAgentHistory(ctx, &histories[worker])
				require.NoError(t, err)
				// The final write is operation 984; later operations only read.
				require.Equal(t, uint64(concurrency*operations+operations-16+2), row.Revision)
				require.Equal(t, payload, row.Payload)
			}
		})
	}
	var integrity string
	require.NoError(t, db.Raw("PRAGMA quick_check").Scan(&integrity).Error)
	require.Equal(t, "ok", integrity)
}
