package storage

import (
	"aladin/backend_v2/internal/service"
	"bytes"
	"encoding/json"
	"testing"
)

func TestShardResourceArchiveRestoresRecordsTombstonesAndReceipts(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	command := service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks"}, Op: "insert", RequestID: "archive-command", Data: json.RawMessage(`{"title":"kept"}`)}
	original, err := h.svc.Mutate(h.ctx, h.target, command)
	if err != nil {
		t.Fatal(err)
	}
	deleted := h.insert(t, "deleted", `{"title":"retained tombstone"}`)
	_, err = h.svc.Mutate(h.ctx, h.target, service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks", ID: deleted.Record.ID}, Op: "delete", RequestID: "archive-delete", BaseRevision: deleted.Record.Revision})
	if err != nil {
		t.Fatal(err)
	}
	var backup bytes.Buffer
	manifest, err := h.repo.ExportResourceData(h.ctx, h.target.ShardID, h.target.Environment, &backup)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Records != 2 || manifest.Receipts != 3 || !bytes.Contains(backup.Bytes(), []byte("retained tombstone")) {
		t.Fatalf("incomplete backup: %+v", manifest)
	}
	_, err = h.repo.RestoreResourceData(h.ctx, h.target.ShardID, h.target.Environment, bytes.NewReader(backup.Bytes()), h.profiles)
	requireResourceCode(t, err, "conflict")
	// Destructive recovery simulation is confined to this unique sandbox shard.
	for _, table := range []string{"shard_resource_records", "shard_resource_receipts"} {
		if _, err := h.pool.Exec(h.ctx, "DELETE FROM "+table+" WHERE shard_id=$1 AND environment=$2", h.target.ShardID, string(h.target.Environment)); err != nil {
			t.Fatal(err)
		}
	}
	damaged := bytes.Replace(backup.Bytes(), []byte(`"title":"kept"`), []byte(`"title":"evil"`), 1)
	if bytes.Equal(damaged, backup.Bytes()) {
		t.Fatal("fault injection did not change archive")
	}
	_, err = h.repo.RestoreResourceData(h.ctx, h.target.ShardID, h.target.Environment, bytes.NewReader(damaged), h.profiles)
	requireResourceCode(t, err, "bad-request")
	var count int
	if err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM shard_resource_records WHERE shard_id=$1`, h.target.ShardID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed restore was not atomic: %d %v", count, err)
	}
	restored, err := h.repo.RestoreResourceData(h.ctx, h.target.ShardID, h.target.Environment, bytes.NewReader(backup.Bytes()), h.profiles)
	if err != nil || restored != manifest {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	snapshot := h.read(t, service.ResourceRequest{Binding: "tasks"})
	if len(snapshot.Records) != 1 || snapshot.Records[0].ID != original.Record.ID {
		t.Fatalf("restored records differ: %+v", snapshot)
	}
	replay, err := h.svc.Mutate(h.ctx, h.target, command)
	if err != nil || replay.Record.ID != original.Record.ID || replay.Record.Revision != original.Record.Revision {
		t.Fatalf("receipt did not survive restore: %+v %v", replay, err)
	}
	var second bytes.Buffer
	after, err := h.repo.ExportResourceData(h.ctx, h.target.ShardID, h.target.Environment, &second)
	if err != nil || after != manifest {
		t.Fatalf("restored checksum differs: %+v %v", after, err)
	}
}
