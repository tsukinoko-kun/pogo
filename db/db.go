package db

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	connPool *pgxpool.Pool
	Q        *Queries
)

func Connect() {
	connectionString, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		_, _ = fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		os.Exit(1)
		return
	}

	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to parse DATABASE_URL")
		os.Exit(1)
		return
	}
	config.MaxConns = 10
	config.MinConns = 2
	config.HealthCheckPeriod = time.Second * 10
	config.MaxConnLifetime = time.Second * 60
	config.MaxConnIdleTime = time.Second * 30

	for range 10 {
		connPool, err = pgxpool.NewWithConfig(context.Background(), config)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "failed to connect to database")
			time.Sleep(time.Second)
			continue
		}

		err = connPool.Ping(context.Background())
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "failed to ping database")
			connPool.Close()
			time.Sleep(time.Second)
			continue
		}

		_, _ = fmt.Fprintln(os.Stdout, "connected to database")

		if err := Migrate(context.Background(), connPool); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "failed to migrate database:", err.Error())
			connPool.Close()
			os.Exit(1)
			return
		}

		Q = &Queries{
			db: connPool,
		}

		return
	}

	_, _ = fmt.Fprintln(os.Stderr, "failed to connect to database after 10 attempts")
	os.Exit(1)
}

func Disconnect() {
	connPool.Close()
}

type TxQueries struct {
	*Queries
	tx  pgx.Tx
	ctx context.Context
}

func (q TxQueries) Close() error {
	if q.db == nil {
		return nil
	}
	if tx, ok := q.db.(pgx.Tx); ok {
		return tx.Rollback(context.Background())
	}
	return errors.New("db was expected to be a pgx.Tx but it wasn't")
}

func (q TxQueries) Commit() error {
	if q.db == nil {
		return errors.New("db is nil")
	}
	if tx, ok := q.db.(pgx.Tx); ok {
		return tx.Commit(context.Background())
	}
	return errors.New("db was expected to be a pgx.Tx but it wasn't")
}

func (q *Queries) Begin(ctx context.Context) (*TxQueries, error) {
	if q.db == nil {
		return nil, errors.New("db is nil")
	}
	switch db := q.db.(type) {
	case *pgxpool.Pool:
		tx, err := db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		q := &Queries{
			db: tx,
		}
		return &TxQueries{
			Queries: q,
			tx:      tx,
			ctx:     ctx,
		}, nil
	case pgx.Tx:
		tx, err := db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		q := &Queries{
			db: tx,
		}
		return &TxQueries{
			Queries: q,
			tx:      tx,
			ctx:     ctx,
		}, nil
	}
	return nil, errors.New("db is neither *pgxpool.Pool nor pgx.Tx")
}

func (q *Queries) FindChange(ctx context.Context, repositoryID int32, search string) (int64, error) {
	changeIds, err := q.findChanges(ctx, repositoryID, search, 2)
	if err != nil {
		return 0, err
	}
	switch len(changeIds) {
	case 0:
		return 0, pgx.ErrNoRows
	case 1:
		return changeIds[0], nil
	default:
		return 0, fmt.Errorf("multiple changes found %#v", changeIds)
	}
}

const (
	changeIdAlphabet = "abcdefhkmnprwxyACDEFHJKLMNPRXY34"
	byteCount        = 10
)

var changeIdEncoding = base32.NewEncoding(changeIdAlphabet).WithPadding(base32.NoPadding)

func (r *Queries) GenerateChangeName(ctx context.Context, repositoryID int32) (string, error) {
	for range 16 {
		src := make([]byte, byteCount)
		_, _ = rand.Read(src)
		changeName := changeIdEncoding.EncodeToString(src)
		// check if it's already taken
		if _, err := r.FindChangeExact(ctx, repositoryID, changeName); err != nil {
			if err == pgx.ErrNoRows {
				// it's available
				return changeName, nil
			}
			return "", errors.Join(errors.New("lookup if change name is taken"), err)
		} else {
			// it's taken
			fmt.Printf("change name %s is taken\n", changeName)
			continue
		}
	}
	return "", errors.New("too many attempts")
}

func UpsertFile(q Querier, ctx context.Context, name string, executable *bool, contentHash []byte, conflict bool) (int64, error) {
	var id int64

	if executable == nil {
		var err error
		id, err = q.getFileWithoutExecutable(ctx, contentHash, name)
		if err != nil {
			if err == pgx.ErrNoRows {
				id, err = q.createFile(ctx, name, strings.HasSuffix(name, ".sh"), contentHash, conflict)
				if err != nil {
					return 0, err
				}
			} else {
				return 0, err
			}
		}
	} else {
		var err error
		id, err = q.getFileWithExecutable(ctx, contentHash, name, *executable)
		if err != nil {
			if err == pgx.ErrNoRows {
				id, err = q.createFile(ctx, name, *executable, contentHash, conflict)
				if err != nil {
					return 0, err
				}
			} else {
				return 0, err
			}
		}
	}

	return id, nil
}
