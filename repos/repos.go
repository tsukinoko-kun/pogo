package repos

import (
	"context"
	"errors"
	"fmt"
	"github.com/tsukinoko-kun/pogo/colors"
	"github.com/tsukinoko-kun/pogo/db"
	"github.com/tsukinoko-kun/pogo/runedrawer"
	"io"
	"math"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nulab/autog"
	"github.com/nulab/autog/graph"
)

type Repo int32

func (r Repo) ID() int32 {
	return int32(r)
}

var (
	ErrRepoNotFound = errors.New("repo not found")
)

func OpenByName(name string) (Repo, error) {
	r, err := db.Q.GetRepoByName(context.Background(), name)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrRepoNotFound
	}
	return Repo(r), err
}

func Open(id int32) Repo {
	return Repo(id)
}

func Create(q db.Querier, name string) (Repo, error) {
	r, err := q.CreateRepo(context.Background(), name)
	return Repo(r), err
}

type LogOptions struct {
	// Ctx is the context to use for the database queries.
	Ctx context.Context
	// Target is the writer to which the log output is written. If Target is nil, the default is os.Stdout.
	Target io.Writer
	// Limit limits the number of commits to show. If Limit is 0 or less, the default limit is used. The default limit is 10.
	Limit int32
	// TimeZone specifies the time zone to use for the change timestamps.
	TimeZone *time.Location
	// Head is the commit to start the log from.
	Head int64
}

type LogChangeInfo struct {
	ID           int64
	Prefix       string
	Name         string
	Description  *string
	Author       string
	Device       string
	Depth        int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	HasConflicts bool
}

func (r Repo) PrintLog(opts LogOptions) error {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	if opts.Target == nil {
		opts.Target = os.Stdout
	}

	tx, err := db.Q.Begin(opts.Ctx)
	if err != nil {
		return errors.Join(errors.New("begin transaction"), err)
	}
	defer tx.Close()

	// construct the graph object from an ordered adjacency list
	var adjacencyList [][]string
	nodeIdChangeMap := make(map[string]LogChangeInfo)

	{
		ancestry, err := tx.GetAncestryOfChange(opts.Ctx, opts.Head, opts.Limit, r.ID())
		if err != nil {
			return errors.Join(errors.New("get ancestry of change"), err)
		}

		if len(ancestry) == 0 {
			return errors.New("no history found")
		}

		if len(ancestry) == 1 {
			c := ancestry[0]
			prefix, err := tx.GetChangePrefix(opts.Ctx, c.ID, r.ID())
			if err != nil {
				prefix = c.Name
			}
			_, _ = fmt.Fprint(opts.Target, colors.Magenta+prefix+colors.Reset)
			_, _ = fmt.Fprintln(opts.Target, colors.BrightBlack+c.Name[len(prefix):]+colors.Reset)
			if c.Description != nil {
				_, _ = fmt.Fprintln(opts.Target, *c.Description)
			} else {
				_, _ = fmt.Fprintln(opts.Target, colors.Green+"(no description set)"+colors.Reset)
			}
			_, _ = fmt.Fprintln(opts.Target, colors.BrightBlack+
				fmt.Sprintf(
					"%s %s",
					c.Author,
					c.UpdatedAt.Time.In(opts.TimeZone).Format(time.DateTime),
				)+colors.Reset,
			)
			return nil
		}

		for _, c := range ancestry {
			stringId := fmt.Sprintf("%d", c.ID)
			conflicts, err := tx.HasChangeConflicts(opts.Ctx, c.ID)
			if err != nil {
				return errors.Join(fmt.Errorf("get change %s conflicts", c.Name), err)
			}
			prefix, err := tx.GetChangePrefix(opts.Ctx, c.ID, r.ID())
			if err != nil {
				return errors.Join(fmt.Errorf("get change %s prefix", c.Name), err)
			}
			nodeIdChangeMap[stringId] = LogChangeInfo{
				c.ID,
				prefix,
				c.Name,
				c.Description,
				c.Author,
				c.Device,
				c.Depth,
				c.CreatedAt.Time.In(opts.TimeZone),
				c.UpdatedAt.Time.In(opts.TimeZone),
				conflicts,
			}
			if c.ParentID != nil {
				adjacencyList = append(adjacencyList, []string{
					stringId,
					fmt.Sprintf("%d", *c.ParentID),
				})
			} else {
				adjacencyList = append(adjacencyList, []string{stringId, "~"})
			}
		}
	}

	if len(adjacencyList) == 0 {
		return errors.New("found history, but generated adjacency list is empty")
	}

	// obtain a graph.Source (here by converting the input to EdgeSlice)
	src := graph.EdgeSlice(adjacencyList)

	// run the default autolayout pipeline
	layout := autog.Layout(
		src,
		autog.WithNodeFixedSize(0, 0),
		autog.WithOrdering(autog.OrderingWMedian),
		autog.WithPositioning(autog.PositioningBrandesKoepf),
		autog.WithEdgeRouting(autog.EdgeRoutingOrtho),
		autog.WithNodeVerticalSpacing(2),
		autog.WithNodeSpacing(4),
		autog.WithLayerSpacing(0),
	)

	drawer := runedrawer.New()

	for _, e := range layout.Edges {
		if e.FromID == "~" || e.ToID == "~" {
			continue
		}
		var spline runedrawer.Spline
		for _, p := range e.Points {
			spline = append(spline, runedrawer.Point{
				X: int(math.Round(p[0])),
				Y: int(math.Round(p[1])),
			})
		}
		drawer.DrawSpline(spline)
	}
	drawer.EncodeCorners()

	changeMinHeight := make(map[int64]int)
	for _, n := range layout.Nodes {
		if n.ID == "~" {
			continue
		}
		x := int(math.Round(n.X))
		y := int(math.Round(n.Y))
		change := nodeIdChangeMap[n.ID]
		if prevY, ok := changeMinHeight[change.ID]; ok {
			changeMinHeight[change.ID] = min(prevY, y)
		} else {
			changeMinHeight[change.ID] = y
		}
		var color string
		if change.HasConflicts {
			color = colors.Red
		} else {
			color = colors.White
		}

		if change.ID == opts.Head {
			drawer.WriteX(x, y, color, "●", colors.Reset)
		} else {
			drawer.WriteX(x, y, color, "○", colors.Reset)
		}
	}

	// write change details
	paddingLeft := drawer.Width() + 2
	var writtenChangeIds []int64
	for _, change := range nodeIdChangeMap {
		if slices.Contains(writtenChangeIds, change.ID) {
			continue
		}
		writtenChangeIds = append(writtenChangeIds, change.ID)

		height := 0
		if minHeight, ok := changeMinHeight[change.ID]; ok {
			height = minHeight
		}

		suffix := change.Name[len(change.Prefix):]
		drawer.WriteX(paddingLeft, height, colors.Magenta, change.Prefix, colors.Reset)
		drawer.WriteX(paddingLeft+len(change.Prefix), height, colors.BrightBlack, suffix, colors.Reset)
		meta := fmt.Sprintf(
			"%s %s",
			change.Author,
			change.UpdatedAt.Format(time.DateTime),
		)
		if change.HasConflicts && change.ID == opts.Head {
			meta += " ⚠️ " + colors.Red + "conflict" + colors.Reset
		}
		drawer.WriteX(paddingLeft+len(change.Name)+1, height+1, colors.BrightBlack, meta, colors.Reset)
		if change.Description != nil {
			drawer.Write(paddingLeft+len(change.Name)+1, height, strFirstLine(*change.Description))
		} else {
			drawer.WriteX(paddingLeft+len(change.Name)+1, height, colors.Green, "(no description set)", colors.Reset)
		}
	}

	_, _ = fmt.Fprintln(opts.Target, drawer.String())

	return nil
}

func strFirstLine(s string) string {
	fl := strings.Split(s, "\n")[0]
	if len(fl) > 80 {
		fl = strings.TrimSpace(fl[:80]) + "…"
	} else {
		fl = strings.TrimSpace(fl)
	}
	return fl
}
