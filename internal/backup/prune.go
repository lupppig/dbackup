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
	Retention time.Duration
	Keep      int
	DBType    string
	DBName    string
	Logger    *logger.Logger
}

func NewPruneManager(s storage.Storage, opts PruneOptions) *PruneManager {
	return &PruneManager{
		storage: s,
		options: opts,
	}
}

func (m *PruneManager) Prune(ctx context.Context) error {
	if m.options.Retention == 0 && m.options.Keep == 0 {
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

	// Keep N newest
	if m.options.Keep > 0 && len(manifests) > m.options.Keep {
		for i := m.options.Keep; i < len(manifests); i++ {
			toDelete[manifests[i].ID] = true
		}
	}

	// Retention time
	if m.options.Retention > 0 {
		now := time.Now()
		for _, man := range manifests {
			if now.Sub(man.CreatedAt) > m.options.Retention {
				toDelete[man.ID] = true
			}
		}
	}

	for id := range toDelete {
		manifestName := manifestMap[id]
		// Determine backup file name from manifest
		// By convention, backupName.manifest
		backupName := strings.TrimSuffix(manifestName, ".manifest")

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
