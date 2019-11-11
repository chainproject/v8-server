package server

import (
	"context"
	"database/sql"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/trusch/v8-server/pkg/api"

	sq "github.com/Masterminds/squirrel"
	"github.com/golang/protobuf/ptypes"
	"gopkg.in/augustoroman/v8.v1"
)

func New(db *sql.DB) api.V8Server {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS scripts(id UUID, name TEXT, content TEXT, created_at TIMESTAMP)`)
	if err != nil {
		panic(err)
	}
	return &v8Server{db: db, isolate: v8.NewIsolate()}
}

type v8Server struct {
	db      *sql.DB
	isolate *v8.Isolate
}

func (s *v8Server) Upload(ctx context.Context, req *api.UploadRequest) (*api.UploadResponse, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(s.db)
	id := uuid.NewV4()
	now := time.Now()
	_, err := psql.Insert("scripts").Columns("id", "name", "content", "created_at").
		Values(id, req.Name, req.Script, now).
		ExecContext(ctx)
	if err != nil {
		return nil, err
	}
	ts, err := ptypes.TimestampProto(now)
	if err != nil {
		return nil, err
	}
	return &api.UploadResponse{
		Id:        id.String(),
		CreatedAt: ts,
	}, nil
}

func (s *v8Server) Run(ctx context.Context, req *api.RunRequest) (*api.RunResponse, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(s.db)
	var script string
	if req.Id != "" {
		row := psql.Select("content").From("scripts").Where(sq.Eq{"id": req.Id}).QueryRowContext(ctx)
		if err := row.Scan(&script); err != nil {
			return nil, err
		}
	} else {
		row := psql.Select("content").From("scripts").Where(sq.Eq{"name": req.Name}).QueryRowContext(ctx)
		if err := row.Scan(&script); err != nil {
			return nil, err
		}
	}
	v8Ctx := s.isolate.NewContext()
	if len(req.Env) > 0 {
		global := v8Ctx.Global()
		for k, v := range req.Env {
			val, err := v8Ctx.Create(v)
			if err != nil {
				return nil, err
			}
			err = global.Set(k, val)
			if err != nil {
				return nil, err
			}
		}
	}
	res, err := v8Ctx.Eval(script, req.Name)
	if err != nil {
		return nil, err
	}
	bs, err := res.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return &api.RunResponse{
		Output: string(bs),
	}, nil
}

func (s *v8Server) List(req *api.ListRequest, srv api.V8_ListServer) error {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(s.db)
	rows, err := psql.Select("id", "name", "created_at").From("scripts").QueryContext(srv.Context())
	if err != nil {
		return err
	}
	for rows.Next() {
		var (
			id        string
			name      string
			createdAt time.Time
		)
		err = rows.Scan(&id, &name, &createdAt)
		if err != nil {
			return err
		}
		ts, err := ptypes.TimestampProto(createdAt)
		if err != nil {
			return err
		}
		err = srv.Send(&api.ListResponse{
			Id:        id,
			Name:      name,
			CreatedAt: ts,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *v8Server) Delete(ctx context.Context, req *api.DeleteRequest) (*api.DeleteResponse, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(s.db)
	var err error
	if req.Name != "" {
		_, err = psql.Delete("scripts").Where(sq.Eq{"name": req.Name}).ExecContext(ctx)
	} else {
		_, err = psql.Delete("scripts").Where(sq.Eq{"id": req.Id}).ExecContext(ctx)
	}
	if err != nil {
		return nil, err
	}
	return &api.DeleteResponse{}, nil
}
