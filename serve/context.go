package serve

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Context struct {
	ctx       context.Context
	userName  string
	machineId string
}

func ContextFromRequest(r *http.Request) (*Context, error) {
	return &Context{
		ctx:       r.Context(),
		userName:  r.Header.Get("X-Pogo-User"),
		machineId: r.Header.Get("X-Pogo-Machine"),
	}, nil
}

func (c Context) HeadName() string {
	return fmt.Sprintf("__head-%s-%s", c.userName, c.machineId)
}

// implement context.Context interface
func (c Context) Deadline() (deadline time.Time, ok bool) {
	return
}

func (c Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c Context) Err() error {
	return c.ctx.Err()
}

func (c Context) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}
