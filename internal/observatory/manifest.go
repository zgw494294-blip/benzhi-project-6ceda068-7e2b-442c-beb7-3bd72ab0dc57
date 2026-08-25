package observatory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

func BuildManifest(aggregate Aggregate, id, actor string, now time.Time) (FrozenManifest, error) {
	if err := EnsureState(aggregate.Task, StateReviewPending); err != nil {
		return FrozenManifest{}, err
	}
	if HasOpenBlockingFindings(aggregate) {
		return FrozenManifest{}, invalid("UNRESOLVED_FINDINGS", "仍存在未关闭的阻断问题")
	}
	active := ActiveRevisions(aggregate)
	if len(active) == 0 {
		return FrozenManifest{}, invalid("EMPTY_MANIFEST", "没有活动修订可供冻结")
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].LogicalPath != active[j].LogicalPath {
			return active[i].LogicalPath < active[j].LogicalPath
		}
		return active[i].ID < active[j].ID
	})
	entries := make([]ManifestEntry, 0, len(active))
	for _, revision := range active {
		entry := ManifestEntry{
			RevisionID: revision.ID, LogicalPath: revision.LogicalPath, ByteSize: revision.ByteSize,
			MediaType: revision.MediaType, SHA256: revision.SHA256,
		}
		entry.EntryDigest = ManifestEntryDigest(entry)
		entries = append(entries, entry)
	}
	return FrozenManifest{
		ID: id, TaskID: aggregate.Task.ID, TaskVersion: aggregate.Task.Version + 1,
		Entries: entries, MerkleRoot: MerkleRoot(entries), FrozenBy: actor, FrozenAt: now.UTC(),
	}, nil
}

func ManifestEntryDigest(entry ManifestEntry) string {
	payload := fmt.Sprintf("manifest-entry-v1\n%s\n%s\n%d\n%s\n%s", entry.LogicalPath, entry.RevisionID, entry.ByteSize, entry.MediaType, entry.SHA256)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func MerkleRoot(entries []ManifestEntry) string {
	if len(entries) == 0 {
		sum := sha256.Sum256([]byte("empty-manifest-v1"))
		return hex.EncodeToString(sum[:])
	}
	level := make([][]byte, len(entries))
	for index, entry := range entries {
		digest, err := hex.DecodeString(ManifestEntryDigest(entry))
		if err != nil {
			digest = make([]byte, sha256.Size)
		}
		level[index] = digest
	}
	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for index := 0; index < len(level); index += 2 {
			left := level[index]
			right := left
			if index+1 < len(level) {
				right = level[index+1]
			}
			payload := append(append([]byte("node-v1\n"), left...), right...)
			sum := sha256.Sum256(payload)
			next = append(next, sum[:])
		}
		level = next
	}
	return hex.EncodeToString(level[0])
}

func VerifyManifest(manifest FrozenManifest) []string {
	reasons := make([]string, 0)
	for index, entry := range manifest.Entries {
		if ManifestEntryDigest(entry) != entry.EntryDigest {
			reasons = append(reasons, fmt.Sprintf("清单条目 %d 摘要不匹配", index))
		}
		if index > 0 {
			previous := manifest.Entries[index-1]
			if previous.LogicalPath > entry.LogicalPath || previous.LogicalPath == entry.LogicalPath && previous.RevisionID > entry.RevisionID {
				reasons = append(reasons, "清单条目顺序不稳定")
			}
		}
	}
	if MerkleRoot(manifest.Entries) != manifest.MerkleRoot {
		reasons = append(reasons, "Merkle 根与清单内容不匹配")
	}
	return reasons
}
