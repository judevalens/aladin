package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/shardv2"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type resourceSQL struct{ args []any }

func (q *resourceSQL) bind(value any) string {
	q.args = append(q.args, value)
	return fmt.Sprintf("$%d", len(q.args))
}
func (q *resourceSQL) field(pointer string) string {
	parts, _ := shardv2.PointerParts(pointer)
	return "(data #> " + q.bind(parts) + "::text[])"
}
func (q *resourceSQL) predicate(p shardv2.Predicate) string {
	if len(p.And) > 0 || len(p.Or) > 0 {
		children, op := p.And, " AND "
		if len(p.Or) > 0 {
			children, op = p.Or, " OR "
		}
		parts := make([]string, 0, len(children))
		for _, child := range children {
			parts = append(parts, q.predicate(child))
		}
		return "(" + strings.Join(parts, op) + ")"
	}
	field := q.field(p.Field)
	if p.Op == "exists" {
		if p.Value == true {
			return field + " IS NOT NULL"
		}
		return field + " IS NULL"
	}
	values := []any{p.Value}
	if p.Op == "in" {
		values = p.Value.([]any)
	}
	parts := []string{}
	for _, value := range values {
		encoded, _ := json.Marshal(value)
		operand := q.bind(encoded) + "::jsonb"
		switch p.Op {
		case "eq", "in":
			parts = append(parts, field+" = "+operand)
		case "gt", "gte", "lt", "lte":
			op := map[string]string{"gt": ">", "gte": ">=", "lt": "<", "lte": "<="}[p.Op]
			parts = append(parts, "(jsonb_typeof("+field+") = 'number' AND "+field+" "+op+" "+operand+")")
		}
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}
func scalarSortKind(schema shardv2.Schema) (string, error) {
	types := []string{}
	switch value := schema["type"].(type) {
	case string:
		types = []string{value}
	case []any:
		for _, item := range value {
			types = append(types, item.(string))
		}
	}
	kind := ""
	for _, typ := range types {
		if typ == "null" {
			continue
		}
		if typ == "integer" {
			typ = "number"
		}
		if kind != "" && kind != typ {
			return "", shardresource.Failure("unsupported-capability", "Mixed scalar types cannot be sorted")
		}
		kind = typ
	}
	if kind == "" {
		kind = "string"
	}
	return kind, nil
}
func resourceQuerySQL(view shardresource.View, offset int) (string, []any, error) {
	// Normalize Go typed predicates before the SQL compiler reads their JSON values.
	raw, err := json.Marshal(view.Query)
	if err != nil {
		return "", nil, err
	}
	if err := json.Unmarshal(raw, &view.Query); err != nil {
		return "", nil, err
	}
	if err := shardv2.ValidateQuery(view.Definition, view.Query); err != nil {
		return "", nil, err
	}
	ns := view.Namespace
	q := resourceSQL{args: []any{ns.UserID, ns.ShardID, string(ns.Environment), ns.Generation, ns.DatasetID}}
	where := `user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND generation=$4 AND dataset_id=$5 AND deleted_at IS NULL`
	if view.ID != "" {
		where += " AND id=" + q.bind(view.ID)
	}
	if view.Query.Where != nil {
		where += " AND " + q.predicate(*view.Query.Where)
	}
	ordering := []string{}
	for _, order := range view.Query.OrderBy {
		schema, err := shardv2.FieldSchema(view.Definition.Schema, order.Field)
		if err != nil {
			return "", nil, err
		}
		kind, err := scalarSortKind(schema)
		if err != nil {
			return "", nil, err
		}
		field := q.field(order.Field)
		direction := "ASC"
		if order.Direction == "desc" {
			direction = "DESC"
		}
		cast := field + " #>> '{}'"
		switch kind {
		case "number":
			cast = "(" + cast + ")::numeric"
		case "boolean":
			cast = "(" + cast + ")::boolean"
		default:
			cast = "(" + cast + ") COLLATE \"C\""
		}
		// JSON null and missing both become SQL NULL, last in BOTH directions.
		ordering = append(ordering, "CASE WHEN "+field+" IS NULL OR "+field+"='null'::jsonb THEN NULL ELSE "+cast+" END "+direction+" NULLS LAST")
	}
	ordering = append(ordering, `id COLLATE "C" ASC`)
	query := `SELECT id,revision::text,schema_version,data FROM shard_resource_records WHERE ` + where + " ORDER BY " + strings.Join(ordering, ",") + " LIMIT " + q.bind(view.Query.Limit+1) + " OFFSET " + q.bind(offset)
	return query, q.args, nil
}

func (r *ShardResourcePostgres) Snapshot(ctx context.Context, view shardresource.View) (shardresource.Page, error) {
	empty := shardresource.Page{}
	if err := r.Authorize(ctx, view); err != nil {
		return empty, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	ns := view.Namespace
	if err := lockResourceNamespace(ctx, tx, ns); err != nil {
		return empty, err
	}
	if err := checkResourceRelease(ctx, tx, ns); err != nil {
		return empty, err
	}
	offset := 0
	if view.Query.Cursor != nil && *view.Query.Cursor != "" {
		token, err := uuid.Parse(*view.Query.Cursor)
		if err != nil {
			return empty, shardresource.Failure("stale-cursor", "Invalid resource cursor")
		}
		err = tx.QueryRow(ctx, `SELECT page_offset FROM shard_resource_cursors WHERE token=$1::uuid AND user_id=$2::uuid AND actor_key=$3 AND shard_id=$4 AND environment=$5 AND view_hash=$6 AND expires_at>now()`, token.String(), ns.UserID, ns.ActorKey, ns.ShardID, string(ns.Environment), view.ViewHash).Scan(&offset)
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, shardresource.Failure("stale-cursor", "Resource cursor expired or belongs to another view")
		}
		if err != nil {
			return empty, err
		}
	}
	query, args, err := resourceQuerySQL(view, offset)
	if err != nil {
		return empty, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return empty, err
	}
	page := shardresource.Page{Records: []shardv2.Record{}}
	for rows.Next() {
		var record shardv2.Record
		if err := rows.Scan(&record.ID, &record.Revision, &record.SchemaVersion, &record.Data); err != nil {
			rows.Close()
			return empty, err
		}
		page.Records = append(page.Records, record)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return empty, err
	}
	if len(page.Records) > view.Query.Limit {
		page.Records = page.Records[:view.Query.Limit]
		if _, err := tx.Exec(ctx, `DELETE FROM shard_resource_cursors WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND expires_at<=now()`, ns.UserID, ns.ShardID, string(ns.Environment)); err != nil {
			return empty, err
		}
		// Reuse an existing cursor for the same page so unchanged live snapshots do
		// not generate new tokens/events on every refresh.
		err = tx.QueryRow(ctx, `SELECT token::text FROM shard_resource_cursors WHERE user_id=$1::uuid AND actor_key=$2 AND shard_id=$3 AND environment=$4 AND view_hash=$5 AND page_offset=$6 AND expires_at>now() ORDER BY expires_at DESC LIMIT 1`, ns.UserID, ns.ActorKey, ns.ShardID, string(ns.Environment), view.ViewHash, offset+len(page.Records)).Scan(&page.NextCursor)
		if errors.Is(err, pgx.ErrNoRows) {
			var count int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM shard_resource_cursors WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3`, ns.UserID, ns.ShardID, string(ns.Environment)).Scan(&count); err != nil {
				return empty, err
			}
			if count >= r.limits.Cursors {
				return empty, shardresource.Failure("quota", "Resource cursor quota exceeded")
			}
			page.NextCursor = uuid.NewString()
			if _, err := tx.Exec(ctx, `INSERT INTO shard_resource_cursors(token,user_id,actor_key,shard_id,environment,view_hash,page_offset,expires_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,now()+interval '15 minutes')`, page.NextCursor, ns.UserID, ns.ActorKey, ns.ShardID, string(ns.Environment), view.ViewHash, offset+len(page.Records)); err != nil {
				return empty, err
			}
		} else if err != nil {
			return empty, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return empty, err
	}
	return page, nil
}
