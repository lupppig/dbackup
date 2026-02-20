package backup

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/manifest"
	"github.com/lupppig/dbackup/internal/storage"
)

type PruneManager struct {
	storage storage.Storage
	options PruneOptions
}

type PruneOptions struct {
	Retention       time.Duration
	Keep            int
	RetentionPolicy RetentionPolicy
	DBType          string
	DBName          string
	Logger          *logger.Logger
}

func NewPruneManager(s storage.Storage, opts PruneOptions) *PruneManager {
	return &PruneManager{
		storage: s,
		options: opts,
	}
}

func (m *PruneManager) Prune(ctx context.Context) error {
	policy := m.options.RetentionPolicy
	if m.options.Retention == 0 && m.options.Keep == 0 &&
		policy.KeepDaily == 0 && policy.KeepWeekly == 0 &&
		policy.KeepMonthly == 0 && policy.KeepYearly == 0 {
		return nil
	}

	// List all manifests
	//  prefix based on DBType and DBName if possible, or just list all backups
	// Based on DedupeStorage, manifests are in backups/
	// Based on BackupManager, they are at the root or target dir.
	// Let's list all .manifest files.
	files, err := m.storage.ListMetadata(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list manifests for pruning: %w", err)
	}

	var manifests []*manifest.Manifest
	manifestMap := make(map[string]string) // manifest name -> data

	for _, file := range files {
		if !strings.HasSuffix(file, ".manifest") {
			continue
		}

		data, err := m.storage.GetMetadata(ctx, file)
		if err != nil {
			continue
		}

		man, err := manifest.Deserialize(data)
		if err != nil {
			continue
		}

		if man.Engine != m.options.DBType {
			continue
		}
		if m.options.DBName != "" && man.DBName != m.options.DBName {
			continue
		}

		manifests = append(manifests, man)
		manifestMap[man.ID] = file
	}

	if len(manifests) == 0 {
		return nil
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].CreatedAt.After(manifests[j].CreatedAt)
	})

	toDelete := make(map[string]bool)

	// 1. Keep N newest
	if m.options.Keep > 0 {
		for i := 0; i < len(manifests) && i < m.options.Keep; i++ {
			toDelete[manifests[i].ID] = false
		}
	}

	// 2. GFS Retention
	if policy.KeepDaily > 0 || policy.KeepWeekly > 0 || policy.KeepMonthly > 0 || policy.KeepYearly > 0 {
		m.applyGFSRetention(manifests, toDelete)
	}

	// 3. Simple Duration Retention (fallback/parallel)
	if m.options.Retention > 0 {
		now := time.Now()
		for _, man := range manifests {
			if _, protected := toDelete[man.ID]; !protected {
				if now.Sub(man.CreatedAt) > m.options.Retention {
					toDelete[man.ID] = true
				}
			}
		}
	}

	// Final pass: if it's not explicitly set to false (keep), it should be true (delete) if it exceeded simple Keep
	if m.options.Keep > 0 {
		for i := m.options.Keep; i < len(manifests); i++ {
			if _, exists := toDelete[manifests[i].ID]; !exists {
				toDelete[manifests[i].ID] = true
			}
		}
	}

	for id, deleteMe := range toDelete {
		if !deleteMe {
			continue
		}
		manifestName := manifestMap[id]
		// Determine backup file name from manifest
		// By convention, backupName.manifest
		backupName := strings.TrimSuffix(manifestName, ".manifest")

		if m.options.Logger != nil {
			m.options.Logger.Info("Pruning old backup", "file", backupName)
		}

		// Delete backup file
		if err := m.storage.Delete(ctx, backupName); err != nil && m.options.Logger != nil {
			m.options.Logger.Warn("Failed to prune backup file", "error", err, "file", backupName)
		}

		// Delete manifest
		if err := m.storage.Delete(ctx, manifestName); err != nil && m.options.Logger != nil {
			m.options.Logger.Warn("Failed to prune manifest", "error", err, "file", manifestName)
		}
	}

	return nil
}

func (m *PruneManager) applyGFSRetention(manifests []*manifest.Manifest, toKeep map[string]bool) {
	policy := m.options.RetentionPolicy

	type bucketKey struct {
		year, month, day, week int
	}

	// Newest first is already sorted.
	// We iterate through and keep the FIRST (newest) backup for each bucket.

	keptDaily, keptWeekly, keptMonthly, keptYearly := 0, 0, 0, 0
	dailyBuckets := make(map[string]bool)
	weeklyBuckets := make(map[string]bool)
	monthlyBuckets := make(map[string]bool)
	yearlyBuckets := make(map[string]bool)

	for _, man := range manifests {
		t := man.CreatedAt
		y, mon, d := t.Date()
		_, w := t.ISOWeek()

		dayKey := fmt.Sprintf("%d-%02d-%02d", y, mon, d)
		weekKey := fmt.Sprintf("%d-W%02d", y, w)
		monthKey := fmt.Sprintf("%d-%02d", y, mon)
		yearKey := fmt.Sprintf("%d", y)

		keepThis := false

		if keptDaily < policy.KeepDaily && !dailyBuckets[dayKey] {
			dailyBuckets[dayKey] = true
			keptDaily++
			keepThis = true
		}
		if keptWeekly < policy.KeepWeekly && !weeklyBuckets[weekKey] {
			weeklyBuckets[weekKey] = true
			keptWeekly++
			keepThis = true
		}
		if keptMonthly < policy.KeepMonthly && !monthlyBuckets[monthKey] {
			monthlyBuckets[monthKey] = true
			keptMonthly++
			keepThis = true
		}
		if keptYearly < policy.KeepYearly && !yearlyBuckets[yearKey] {
			yearlyBuckets[yearKey] = true
			keptYearly++
			keepThis = true
		}

		if keepThis {
			toKeep[man.ID] = false // false means DON'T delete
		}
	}
}
